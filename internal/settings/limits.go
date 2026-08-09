package settings

// Limit is how many items one family of pages serves: the page a handler
// returns when the caller asks for no limit, and the most it ever returns
// when the caller asks for more.
type Limit int

// Resolve turns the limit a caller asked for into the one served: the stored
// limit when nothing was asked for, and when more than it was.
func (l Limit) Resolve(requested int) int {
	if requested <= 0 || requested > int(l) {
		return int(l)
	}
	return requested
}

// Resolve64 is Resolve for the handlers that page in int64.
func (l Limit) Resolve64(requested int64) int64 {
	if requested <= 0 || requested > int64(l) {
		return int64(l)
	}
	return requested
}
