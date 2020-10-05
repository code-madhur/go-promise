package main

import (
	"errors"
	"flag"
	"fmt"
)

// Available states for a promises
const (
	PENDING   = 0
	FULFILLED = 1
	REJECTED  = 2
)

// Promise struct
type Promise struct {
	// state pending 0, fulfilled 1, rejected 2
	state          int
	executor       func(resolve func(interface{}), reject func(error))
	resolveChannel chan interface{} // values are passed by resolve and read by Then, Catch and Finally
	rejectChannel  chan error       // values are passed by reject and read by Then, Catch and Finally
	result         interface{}      // Holds the result values down the promise chain
	err            error            // Holds the error values down the chain
}

// New - returns a new promise object
func New(executor func(resolve func(interface{}), reject func(error))) *Promise {
	promise := &Promise{
		state:          PENDING,
		executor:       executor,
		resolveChannel: make(chan interface{}, 1),
		rejectChannel:  make(chan error, 1),
		result:         nil,
		err:            nil,
	}

	go func() {
		defer promise.handlePanic()
		promise.executor(promise.resolve, promise.reject)
	}()

	return promise
}

func (promise *Promise) handlePanic() {
	// Recover any error messages from panic during execution
	e := recover()
	if e != nil {
		switch err := e.(type) {
		case nil:
			promise.reject(fmt.Errorf("panic recovery with nil error"))
		case error:
			promise.reject(fmt.Errorf("panic recovery with error: %s", err.Error()))
		default:
			promise.reject(fmt.Errorf("panic recovery with unknown error: %s", fmt.Sprint(err)))
		}
	}
}

// Reject - Function to return a rejected promise
func Reject(err error) *Promise {
	return New(func(resolve func(interface{}), reject func(error)) {
		reject(err)
	})
}

// Rejects a promise with given error
func (promise *Promise) reject(err error) {
	if promise.state != PENDING {
		return
	}

	promise.state = REJECTED
	promise.rejectChannel <- err
}

// Resets the promise state to PENDING
func (promise *Promise) resetState() {
	promise.state = PENDING
}

// Resolve - function to return a resolved promise
func Resolve(value interface{}) *Promise {
	return New(func(resolve func(interface{}), reject func(error)) {
		resolve(value)
	})
}

// Resolves a promise with given value.
func (promise *Promise) resolve(resolution interface{}) {
	if promise.state != PENDING {
		return
	}

	switch result := resolution.(type) {
	case *Promise:
		flattenedResult, err := result.Await()
		if err != nil {
			promise.reject(err)
			return
		}
		promise.resolveChannel <- flattenedResult
	default:
		promise.resolveChannel <- result
	}

	promise.state = FULFILLED
}

// Then - Appends fulfillment and rejection handlers to the promise, and returns
// a new promise resolving to the return value of the called handler, or
// to its original settled value if the promise was not handled
func (promise *Promise) Then(OnFulfill func(data interface{}) interface{}, OnRejection func(err error) error) *Promise {
	return New(func(resolve func(interface{}), reject func(error)) {
		func() {
			select {
			case result := <-promise.resolveChannel:
				promise.resetState()
				resolve(OnFulfill(result))
			case err := <-promise.rejectChannel:
				reject(OnRejection(err))
			}
		}()
	})
}

// Catch - Appends a handler to the promise, and returns a new promise that is resolved
// when the original promise is resolved. The handler is called when the promise is settled,
// whether fulfilled or rejected.
func (promise *Promise) Catch(OnRejection func(err error) error) *Promise {
	return New(func(resolve func(interface{}), reject func(error)) {
		select {
		case result := <-promise.resolveChannel:
			resolve(result)
		case err := <-promise.rejectChannel:
			reject(OnRejection(err))
			return
		}
	})
}

// Finally - When the promise is settled, i.e either fulfilled or rejected,
// the specified callback function is executed. This provides a way for code to be
// run whether the promise was fulfilled successfully or rejected once the Promise has been dealt with.
func (promise *Promise) Finally(onFinally func() interface{}) *Promise {
	return New(func(resolve func(interface{}), reject func(error)) {
		select {
		case err := <-promise.rejectChannel:
			reject(err)
		case result := <-promise.resolveChannel:
			resolve(result)
		}
		onFinally()
	})
}

// Await - function to wait for either a result or error to happen on callbacks execution
func (promise *Promise) Await() (interface{}, error) {
	select {
	case result := <-promise.resolveChannel:
		promise.result = result
	case err := <-promise.rejectChannel:
		promise.err = err
	}
	return promise.result, promise.err
}

var testNum int

func init() {
	flag.IntVar(&testNum, "testNum", 3, "Num to be tested for equality with 3")
	flag.Parse()
}
func main() {
	var p = New(func(resolve func(interface{}), reject func(error)) {
		fmt.Println(testNum)
		// If condition passes resolve the promise
		if testNum == 3 {
			resolve(testNum)
			return
		}

		// If condition fails reject the promise.
		if testNum != 3 {
			reject(errors.New(fmt.Sprintf("%d is not equal to 3", testNum)))
			return
		}
	}).
		Then(func(data interface{}) interface{} {
			fmt.Println("Current value is:", data)
			return data.(int) + 1
		},
			func(err error) error {
				return err
			}).
		Then(func(data interface{}) interface{} {
			fmt.Println("Current value is:", data)
			return nil
		},
			func(err error) error {
				return err
			}).
		Catch(func(error error) error {
			fmt.Println("I am in catch block")
			fmt.Println(error.Error())
			return nil
		}).
		Finally(func() interface{} {
			fmt.Println("Finally its over")
			return nil
		})

	p.Await()
}
