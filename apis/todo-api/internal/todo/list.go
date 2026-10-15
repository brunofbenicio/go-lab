package todo

// ListOptions narrows and pages the result of Store.List.
type ListOptions struct {
	// Done keeps only todos whose done flag matches. Nil keeps both.
	Done *bool
	// Query keeps only todos whose title or notes contain it, ignoring case.
	Query string
	// Limit is the maximum number of items returned. Zero means no limit.
	Limit int
	// Offset is the number of matching items to skip.
	Offset int
}

// ListResult is one page of todos plus the size of the whole match.
type ListResult struct {
	// Items is the requested page, ordered by ID.
	Items []Todo
	// Total counts every todo that matched the filters, ignoring paging.
	Total int
}
