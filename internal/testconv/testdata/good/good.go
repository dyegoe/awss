package good

type Results struct{}

func (r *Results) GetRows() []string { return nil }

func (r *Results) collect() {}

func parseRow() {}

func New() *Results { return &Results{} }

const pageSize = 10
