package jsonrpc2_test

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/rpc"

	"github.com/powerman/rpc-codec/jsonrpc2"
)

// A server wishes to export an object of type ExampleSvc:
type ExampleSvc struct{}

// Method with positional params.
func (*ExampleSvc) Sum(vals [2]int, res *int) error {
	*res = vals[0] + vals[1]
	return nil
}

// Method with positional params.
func (*ExampleSvc) SumAll(vals []int, res *int) error {
	for _, v := range vals {
		*res += v
	}
	return nil
}

// Method with named params.
func (*ExampleSvc) MapLen(m map[string]int, res *int) error {
	*res = len(m)
	return nil
}

type NameArg struct{ Fname, Lname string }
type NameRes struct{ Name string }

// Method with named params.
func (*ExampleSvc) FullName(t NameArg, res *NameRes) error {
	*res = NameRes{t.Fname + " " + t.Lname}
	return nil
}

// Method returns error with code -32000.
func (*ExampleSvc) Err1(struct{}, *struct{}) error {
	return errors.New("some issue")
}

// Method returns error with code 42.
func (*ExampleSvc) Err2(struct{}, *struct{}) error {
	return jsonrpc2.NewError(42, "some issue")
}

// Method returns error with code 42 and extra error data.
func (*ExampleSvc) Err3(struct{}, *struct{}) error {
	return &jsonrpc2.Error{42, "some issue", []string{"one", "two"}}
}

func Example() {
	// Server export an object of type ExampleSvc.
	rpc.Register(&ExampleSvc{})

	// Server provide a TCP transport.
	lnTCP, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	defer lnTCP.Close()
	go func() {
		for {
			conn, err := lnTCP.Accept()
			if err != nil {
				return
			}
			go jsonrpc2.ServeConn(conn)
		}
	}()

	// Server provide a HTTP transport on /rpc endpoint.
	http.Handle("/rpc", jsonrpc2.HTTPHandler(nil))
	lnHTTP, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	defer lnHTTP.Close()
	go http.Serve(lnHTTP, nil)

	// Client use TCP transport.
	clientTCP, err := jsonrpc2.Dial("tcp", lnTCP.Addr().String())
	if err != nil {
		panic(err)
	}
	defer clientTCP.Close()

	// Client use HTTP transport.
	clientHTTP := jsonrpc2.NewHTTPClient("http://" + lnHTTP.Addr().String() + "/rpc")
	defer clientHTTP.Close()

	var reply int

	// Synchronous call using positional params and TCP.
	err = clientTCP.Call("ExampleSvc.Sum", [2]int{3, 5}, &reply)
	fmt.Printf("Sum(3,5)=%d\n", reply)

	// Synchronous call using positional params and HTTP.
	err = clientHTTP.Call("ExampleSvc.SumAll", []int{3, 5, -2}, &reply)
	fmt.Printf("SumAll(3,5,-2)=%d\n", reply)

	// Asynchronous call using named params and TCP.
	startCall := clientTCP.Go("ExampleSvc.MapLen",
		map[string]int{"a": 10, "b": 20, "c": 30}, &reply, nil)
	replyCall := <-startCall.Done
	fmt.Printf("MapLen({a:10,b:20,c:30})=%d\n", *replyCall.Reply.(*int))

	// Notification using named params and HTTP.
	clientHTTP.Notify("ExampleSvc.FullName", NameArg{"First", "Last"})

	// Correct error handling.
	err = clientTCP.Call("ExampleSvc.Err1", nil, nil)
	if err == rpc.ErrShutdown || err == io.ErrUnexpectedEOF {
		fmt.Printf("Err1(): %q\n", err)
	} else if err != nil {
		rpcerr := jsonrpc2.ServerError(err)
		fmt.Printf("Err1(): code=%d msg=%q data=%v\n", rpcerr.Code, rpcerr.Message, rpcerr.Data)
	}

	err = clientHTTP.Call("ExampleSvc.Err3", nil, nil)
	if err == rpc.ErrShutdown || err == io.ErrUnexpectedEOF {
		fmt.Printf("Err3(): %q\n", err)
	} else if err != nil {
		rpcerr := jsonrpc2.ServerError(err)
		fmt.Printf("Err3(): code=%d msg=%q data=%v\n", rpcerr.Code, rpcerr.Message, rpcerr.Data)
	}

	// Output:
	// Sum(3,5)=8
	// SumAll(3,5,-2)=6
	// MapLen({a:10,b:20,c:30})=3
	// Err1(): code=-32000 msg="some issue" data=<nil>
	// Err3(): code=42 msg="some issue" data=[one two]
}
