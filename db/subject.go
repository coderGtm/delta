package db

// SubjectID returns the user's ID in string form, allowing User to act as a
// request subject.
func (u User) SubjectID() string { return u.ID.String() }
