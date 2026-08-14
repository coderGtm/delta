package db

func (u User) SubjectID() string { return u.ID.String() }
