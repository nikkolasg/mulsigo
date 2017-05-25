package client

type Object struct {
}

func NewTextObject(text string) *Object {
	return nil
}

func NewLocalFileObject(path string) (*Object, error) {
	return nil, nil
}

func NewRemoteFileObject(url string) (*Object, error) {
	return nil, nil
}

// ObjectHandler is an interface allowing one to provide custom verification
// function on any object to be signed. A default impl. is provided which always
// accept to sign. Another one can be provided that ask to the user on the
// command line if it's OK or not etc...
// Implementations can use in conjunctions any  FileGrabber implementation if
// the Object is a RemoteFile object.
type ObjectHandler interface {
	Accept(*Object) bool
}

// defaultHandler is an implementation of ObjectHandler that always accept any
// object passed to it.
type defaultHandler struct {
}

func (dh *defaultHandler) Accept(o *Object) bool {
	return true
}
