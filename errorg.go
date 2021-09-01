package main

type errPathObjHolder struct{}

func newError(values ...interface{}) *Error {
	return NewErr(values...).WithPathObj(errPathObjHolder{})
}
