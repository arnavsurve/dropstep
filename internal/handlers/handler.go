package handlers

type Handler interface {
	Validate() error
	Run() error
}
