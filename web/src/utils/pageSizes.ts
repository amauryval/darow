// The request sizes this client asks for, its own display choices — never a
// read of the operator's settings. The server serves any client, each free to
// page as it likes: it resolves what is asked against its own limits and
// answers with the smaller of the two, so these are requests, not assumptions
// (spec/front §2).
//
// The one thing never decided here is a constraint the server *enforces* (the
// upload cap, the extension whitelist): those are read from it.

// A catalogue page: tables, sources, the SQL reference panel.
export const LIST_PAGE_SIZE = 50

// One chunk of a map query, sized for a rendering that draws page by page
// rather than for a screenful of rows.
export const MAP_FEATURES_PAGE_SIZE = 10_000
