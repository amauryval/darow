// Package observer notifies callbacks registered at wiring time of an item's
// lifecycle events. It is in-memory only: it runs after the write it reports
// has been committed, and it touches neither the database nor the disk.
package observer

// Observer fans an observed item out to the callbacks registered on it.
// Generic over the observed type, so it carries no domain vocabulary — one
// Observer[storage.Metadata] reports a table's removal, one
// Observer[storage.Settings] reports a settings update, each wired
// independently by core.NewServices.
//
// Registration is not safe for concurrent use, and deliberately so: every
// callback is registered while the services are wired, none once requests are
// in flight, exactly as duckdb/catalog.Catalog.Register already works.
type Observer[T any] struct {
	listeners []func(T)
}

// On registers fn, to be called with every item Notify is given afterwards.
func (o *Observer[T]) On(fn func(T)) {
	o.listeners = append(o.listeners, fn)
}

// Notify calls every registered callback with item.
//
// It returns nothing. The event it reports is already committed by the time
// this runs, so a callback has nothing left to veto, and reporting a failed
// cache eviction as the operation's error would answer 5xx for a write that
// in fact succeeded.
func (o *Observer[T]) Notify(item T) {
	for _, fn := range o.listeners {
		fn(item)
	}
}
