package players

type Track struct {
	Title  string
	Artist string
	Album  string
}

type CurrentStatus struct {
	Duration int
	Elapsed  int
	State    string
}

// Status is a struct for encoding the current state of the player
type Status struct {
	Track
	CurrentStatus
}
