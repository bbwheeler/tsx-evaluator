package leadership

// Executive represents a company executive.
type Executive struct {
	Name        string
	Title       string
	Age         int
	YearOfBirth int
}

// LeadershipData holds the data needed to calculate a leadership score.
type LeadershipData struct {
	Executives []Executive
	CEO        *Executive
	CFO        *Executive
	AvgTenure  float64
	CEOAge     int
	CFOAge     int
}
