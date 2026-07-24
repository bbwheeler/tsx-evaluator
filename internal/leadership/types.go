package leadership

// Executive represents a company executive from the FMP API.
type Executive struct {
	Name          string  `json:"name"`
	Title         string  `json:"title"`
	Age           int     `json:"age"`
	Gender        string  `json:"gender"`
	YearOfBirth   int     `json:"yearOfBirth"`
	Compensation  float64 `json:"compensation"`
	Currency      string  `json:"currency"`
	Since         string  `json:"since"`
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
