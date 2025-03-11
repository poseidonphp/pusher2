package dispatcher

type Contract interface {
	Dispatch()
	ListenForEvents()
}
