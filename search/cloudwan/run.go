/*
Copyright © 2022 Dyego Alexandre Eugenio github@dyego.com.br

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cloudwan

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"sync"

	"github.com/dyegoe/awss/common"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/networkmanager"
	"github.com/aws/aws-sdk-go-v2/service/networkmanager/types"
)

// routeWorkers caps the route table calls of one core network running at the same time.
//
// AWS has no call that reads several route tables at once: a core network has one route table per
// segment and edge location, and per network function group and edge location, and each is read on
// its own. The cap keeps a large core network from being throttled.
const routeWorkers = 5

// search is one kind of Cloud WAN route search: the route tables it reads, how it reads one, and
// the hint of an AccessDenied.
type search[R comparable] struct {
	tables func(coreNetworkID string, c *types.CoreNetwork) []routeTable
	read   func(ctx context.Context, cn *coreNetwork, c *routeCall) ([]R, error)
	denied string
}

// coreNetworksAPI is the part of the Network Manager client that lists and reads core networks.
type coreNetworksAPI interface {
	networkmanager.ListCoreNetworksAPIClient
	coreNetworkAPI
}

// run searches every core network of q with client, and returns the rows without duplicates and
// the error messages. A failing core network is reported and the run goes on with the next.
func (s *search[R]) run(
	ctx context.Context, client coreNetworksAPI, q *query, accountNames map[string]string,
) (rows []R, msgs []string) {
	nets, err := CoreNetworks(ctx, client, q.coreNetworks)
	if err != nil {
		return nil, []string{err.Error()}
	}
	for i := range nets {
		cn, tables, loadMsgs := loadCoreNetwork(ctx, client, &nets[i], accountNames, s.tables)
		msgs = append(msgs, loadMsgs...)
		if len(tables) == 0 {
			continue
		}
		read := func(ctx context.Context, c *routeCall) ([]R, error) { return s.read(ctx, cn, c) }
		callRows, callMsgs := runCalls(ctx, cn.id, calls(tables, q), read, s.denied)
		rows = appendUnique(rows, callRows)
		msgs = append(msgs, callMsgs...)
	}
	return rows, msgs
}

// tableReader reads the rows of one call: the routes of one route table, or its BGP routes.
type tableReader[R comparable] func(ctx context.Context, c *routeCall) ([]R, error)

// runCalls makes the calls of one core network with read, at most routeWorkers at once, and
// returns the rows of every call in call order and the error messages.
//
// The first AccessDenied stops the core network: the profile is not the core network owner, or
// lacks the permission, and every other call would be denied too. It is reported once, with
// denied. When parent ends first, the calls not made are reported, so partial rows never look
// complete.
func runCalls[R comparable](
	parent context.Context, coreNetworkID string, cs []routeCall, read tableReader[R], denied string,
) (rows [][]R, msgs []string) {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()

	rows = make([][]R, len(cs))
	errs := make([]error, len(cs))
	sem := make(chan struct{}, routeWorkers)
	var wg sync.WaitGroup
	for i := range cs {
		if !acquire(ctx, sem) {
			break
		}
		wg.Go(func() {
			defer func() { <-sem }()
			rows[i], errs[i] = read(ctx, &cs[i])
			if isAccessDenied(errs[i]) {
				cancel()
			}
		})
	}
	wg.Wait()

	msgs, wasDenied := callErrors(coreNetworkID, cs, errs, denied)
	if !wasDenied && parent.Err() != nil {
		msgs = append(msgs, fmt.Sprintf("stopped before reading every route table of core network %s: %v",
			coreNetworkID, parent.Err()))
	}
	return rows, msgs
}

// acquire takes a slot of sem, and reports false when ctx ends first.
func acquire(ctx context.Context, sem chan struct{}) bool {
	if ctx.Err() != nil {
		return false
	}
	select {
	case sem <- struct{}{}:
		return true
	case <-ctx.Done():
		return false
	}
}

// appendUnique appends the rows of every call to data in call order, skipping the rows already in
// data or seen before: a route can match a CIDR in several modes.
func appendUnique[R comparable](data []R, rows [][]R) []R {
	seen := make(map[R]bool, len(data))
	for i := range data {
		seen[data[i]] = true
	}
	for _, callRows := range rows {
		for i := range callRows {
			if !seen[callRows[i]] {
				seen[callRows[i]] = true
				data = append(data, callRows[i])
			}
		}
	}
	return data
}

// callErrors returns the error messages of the calls in call order, and reports whether one was
// AccessDenied: that one is reported once for the core network, with the hint denied, without the
// calls it cancelled. A call stopped by the end of the parent context is left to runCalls, which
// reports it once.
func callErrors(coreNetworkID string, cs []routeCall, errs []error, denied string) ([]string, bool) {
	for _, err := range errs {
		if isAccessDenied(err) {
			return []string{fmt.Sprintf("access denied reading the routes of core network %s: %s: %v",
				coreNetworkID, denied, err)}, true
		}
	}
	var msgs []string
	for i, err := range errs {
		if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			continue
		}
		t := &cs[i].table
		msgs = append(msgs, fmt.Sprintf("error reading the routes of %s at %s: %v", t.label(), t.edge, err))
	}
	return msgs, false
}

// longestMatches returns the routes whose prefix, given by prefix, is the longest prefix covering
// cidr: the routes the traffic to cidr takes. A route without a prefix, such as a prefix list
// route, never matches.
//
// AWS answers LongestPrefixMatches with "ValidationException: Invalid input. Filter value is
// invalid.", so a longest call reads the whole route table and awss picks the match.
func longestMatches[R any](all []R, cidr string, prefix func(*R) *string) []R {
	want, err := netip.ParsePrefix(cidr)
	if err != nil {
		return nil
	}
	best := -1
	var out []R
	for i := range all {
		p, err := netip.ParsePrefix(aws.ToString(prefix(&all[i])))
		if err != nil || p.Bits() > want.Bits() || !p.Contains(want.Addr()) || p.Bits() < best {
			continue
		}
		if p.Bits() > best {
			best, out = p.Bits(), nil
		}
		out = append(out, all[i])
	}
	return out
}

// tableKey is where a row sits in route table order: core network, segment, edge location, prefix
// by address, then rank, such as the installed path first.
type tableKey struct {
	coreNetwork, segment, edge string
	prefix                     common.CIDR
	rank                       int
}

// compare orders two keys in route table order.
func (k *tableKey) compare(o *tableKey) int {
	if c := cmp.Or(
		cmp.Compare(k.coreNetwork, o.coreNetwork), cmp.Compare(k.segment, o.segment), cmp.Compare(k.edge, o.edge),
	); c != 0 {
		return c
	}
	switch {
	case common.CIDRLess(string(k.prefix), string(o.prefix)):
		return -1
	case common.CIDRLess(string(o.prefix), string(k.prefix)):
		return 1
	}
	return cmp.Compare(k.rank, o.rank)
}

// sortRows sorts data by the struct field, keeping the rows equal on it in route table order, so
// the default sort by segment reads as route tables.
func sortRows[R any](data []R, field string, key func(*R) tableKey) {
	slices.SortStableFunc(data, func(a, b R) int {
		ka, kb := key(&a), key(&b)
		return ka.compare(&kb)
	})
	common.SortByField(data, field)
}
