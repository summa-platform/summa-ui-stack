// https://gist.github.com/ciaranarcher/abccf50cb37645ca27fa
// https://gist.github.com/joeybloggs/ccb8f57b46f770f8a10a
package main

import "net/http"

// LogResponseWritter wraps the standard http.ResponseWritter allowing for more
// verbose logging
type LogResponseWritter struct {
	status int
	size   int
	http.ResponseWriter
}

// func NewMyResponseWriter(res http.ResponseWriter) *MyResponseWriter {
// 	// Default the status code to 200
// 	return &MyResponseWriter{200, res}
// }

// Status provides an easy way to retrieve the status code
func (w *LogResponseWritter) Status() int {
	return w.status
}

// Size provides an easy way to retrieve the response size in bytes
func (w *LogResponseWritter) Size() int {
	return w.size
}

// Header returns & satisfies the http.ResponseWriter interface
func (w *LogResponseWritter) Header() http.Header {
	return w.ResponseWriter.Header()
}

// Write satisfies the http.ResponseWriter interface and
// captures data written, in bytes
func (w *LogResponseWritter) Write(data []byte) (int, error) {

	written, err := w.ResponseWriter.Write(data)
	w.size += written

	return written, err
}

// WriteHeader satisfies the http.ResponseWriter interface and
// allows us to cach the status code
func (w *LogResponseWritter) WriteHeader(statusCode int) {

	w.status = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}
