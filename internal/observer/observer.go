// Package observer notifies callbacks registered at wiring time of an item's
// lifecycle events. It is in-memory only: it runs after the write it reports
// has been committed, and it touches neither the database nor the disk.
package observer

// Observer fans an observed item out to the callbacks registered on it,
// generic over the observed type so that it carries no domain vocabulary.
//
// Registration is deliberately not safe for concurrent use: every callback is
// registered while the services are wired, none once requests are in flight.
type Observer[T any] struct {
	listeners []func(T)
}

// On registers fn, to be called with every item Notify is given afterwards.
func (o *Observer[T]) On(fn func(T)) {
	o.listeners = append(o.listeners, fn)
}

// Notify calls every registered callback with item. It returns nothing: the
// event is already committed, so a callback has nothing left to veto.
func (o *Observer[T]) Notify(item T) {
	for _, fn := range o.listeners {
		fn(item)
	}
}
