package apm

//var (
//	std = New()
//)
//
//func SetApplication(app *newrelic.Application) {
//	std.SetApplication(app)
//}
//
//func NoticeError(ctx context.Context, err error) {
//	std.NoticeError(ctx, err)
//}
//
//func StartTransaction(ctx context.Context, name string) (*NewRelicTransaction, context.Context) {
//	return std.StartTransaction(ctx, name)
//}
//
//func StartWebTransaction(name string, r *http.Request, w http.ResponseWriter) (*NewRelicTransaction, *http.Request, http.ResponseWriter) {
//	return std.StartWebTransaction(name, r, w)
//}
//
//type NewRelicApm struct {
//	application *newrelic.Application
//}
//
//func New() *NewRelicApm {
//	return &NewRelicApm{}
//}
//
//func (a *NewRelicApm) SetApplication(app *newrelic.Application) {
//	a.application = app
//}
//
//func (a *NewRelicApm) NoticeError(ctx context.Context, err error) {
//	txn := newrelic.FromContext(ctx)
//	txn.NoticeError(err)
//}
//
//func (a *NewRelicApm) StartTransaction(ctx context.Context, name string) (*NewRelicTransaction, context.Context) {
//	inner := a.createTransaction(name)
//	c := newrelic.NewContext(ctx, inner)
//
//	return NewTransaction(inner), c
//}
//
//func (a *NewRelicApm) StartWebTransaction(name string, r *http.Request, w http.ResponseWriter) (*NewRelicTransaction, *http.Request, http.ResponseWriter) {
//	inner := a.createTransaction(name)
//
//	// Set the transaction as a web request, gather attributes based on the
//	// request, and read incoming distributed trace headers.
//	inner.SetWebRequestHTTP(r)
//
//	// Prepare to capture attributes, errors, and headers from the
//	// response.
//	w = inner.SetWebResponse(w)
//
//	// Add the NewRelicTransaction to the http.Request's Context.
//	r = newrelic.RequestWithTransactionContext(r, inner)
//
//	// Encapsulate NewRelicTransaction
//	txn := NewTransaction(inner)
//
//	return txn, r, w
//}
//
//func (a *NewRelicApm) createTransaction(name string) *newrelic.Transaction {
//	return a.application.StartTransaction(name)
//}

//type NewRelicTransaction struct {
//	txn *newrelic.Transaction
//}
//
//func NewTransaction(inner *newrelic.Transaction) *NewRelicTransaction {
//	return &NewRelicTransaction{inner}
//}
//
//func (t *NewRelicTransaction) End() {
//	t.txn.End()
//}
