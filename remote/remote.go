// support for generic Remote Object services over sockets
// including a socket wrapper that can drop and/or delay messages arbitrarily
// works with any* objects that can be gob-encoded for serialization
//
// the LeakySocket wrapper for net.Conn is provided in its entirety, and should
// not be changed, though you may extend it with additional helper functions as
// desired.  it is used directly by the test code.
//
// the RemoteObjectError type is also provided in its entirety, and should not
// be changed.
//
// suggested RequestMsg and ReplyMsg types are included to get you started,
// but they are only used internally to the remote library, so you can use
// something else if you prefer
//
// the Service type represents the callee that manages remote objects, invokes
// calls from callers, and returns suitable results and/or remote errors
//
// the StubFactory converts a struct of function declarations into a functional
// caller stub by automatically populating the function definitions.
//
// USAGE:
// the desired usage of this library is as follows (not showing all error-checking
// for clarity and brevity):
//
//  example ServiceInterface known to both client and server, defined as
//  type ServiceInterface struct {
//      ExampleMethod func(int, int) (int, remote.RemoteObjectError)
//  }
//
//  1. server-side program calls NewService with interface and connection details, e.g.,
//     obj := &ServiceObject{}
//     srvc, err := remote.NewService(&ServiceInterface{}, obj, 9999, true, true)
//
//  2. client-side program calls StubFactory, e.g.,
//     stub := &ServiceInterface{}
//     err := StubFactory(stub, 9999, true, true)
//
//  3. client makes calls, e.g.,
//     n, roe := stub.ExampleMethod(7, 14736)
//
//
//
//
//
// TODO *** here's what needs to be done for Lab 2:
//  1. create the Service type and supporting functions, including but not
//     limited to: NewService, Start, Stop, IsRunning, and GetCount (see below)
//	TODO *** NESTED STRUCTS, OTHER ADDITIONAL TYPES, POINTERS (IF NEEDED)

//  2. create the StubFactory which uses reflection to transparently define each
//     method call in the client-side stub (see below)
//

package remote

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"math/rand"
	"net"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"
)

// LeakySocket is a wrapper for a net.Conn connection that emulates
// transmission delays and random packet loss. it has its own send
// and receive functions that together mimic an unreliable connection
// that can be customized to stress-test remote service interactions.
type LeakySocket struct {
	s         net.Conn
	isLossy   bool
	lossRate  float32
	msTimeout int
	usTimeout int
	isDelayed bool
	msDelay   int
	usDelay   int
}

// NewLeakySocket -- builder for a LeakySocket given a normal socket and indicators
// of whether the connection should experience loss and delay.
// uses default loss and delay values that can be changed using setters.
func NewLeakySocket(conn net.Conn, lossy bool, delayed bool) *LeakySocket {
	ls := &LeakySocket{}
	ls.s = conn
	ls.isLossy = lossy
	ls.isDelayed = delayed
	ls.msDelay = 2
	ls.usDelay = 0
	ls.msTimeout = 500
	ls.usTimeout = 0
	ls.lossRate = 0.05

	return ls
}

// SendObject -- send a byte-string over the socket mimicking unreliability.
// delay is emulated using time.Sleep, packet loss is emulated using RNG
// coupled with time.Sleep to emulate a timeout
func (ls *LeakySocket) SendObject(obj []byte) (bool, error) {
	if obj == nil {
		return true, nil
	}

	if ls.s != nil {
		rand.Seed(time.Now().UnixNano())
		if ls.isLossy && rand.Float32() < ls.lossRate {
			time.Sleep(time.Duration(ls.msTimeout)*time.Millisecond + time.Duration(ls.usTimeout)*time.Microsecond)
			return false, nil
		} else {
			if ls.isDelayed {
				time.Sleep(time.Duration(ls.msDelay)*time.Millisecond + time.Duration(ls.usDelay)*time.Microsecond)
			}
			_, err := ls.s.Write(obj)
			if err != nil {
				return false, errors.New("SendObject Write error: " + err.Error())
			}
			return true, nil
		}
	}
	return false, errors.New("SendObject failed, nil socket")
}

// RecvObject -- receive a byte-string over the socket connection.
// no significant change to normal socket receive.
func (ls *LeakySocket) RecvObject() ([]byte, error) {
	if ls.s != nil {
		buf := make([]byte, 4096)
		n := 0
		var err error
		for n <= 0 {
			n, err = ls.s.Read(buf)
			if n > 0 {
				return buf[:n], nil
			}
			if err != nil {
				if err != io.EOF {
					return nil, errors.New("RecvObject Read error: " + err.Error())
				}
			}
		}
	}
	return nil, errors.New("RecvObject failed, nil socket")
}

// SetDelay -- enable/disable emulated transmission delay and/or change the delay parameter
func (ls *LeakySocket) SetDelay(delayed bool, ms int, us int) {
	ls.isDelayed = delayed
	ls.msDelay = ms
	ls.usDelay = us
}

// SetTimeout -- change the emulated timeout period used with packet loss
func (ls *LeakySocket) SetTimeout(ms int, us int) {
	ls.msTimeout = ms
	ls.usTimeout = us
}

// SetLossRate -- enable/disable emulated packet loss and/or change the loss rate
func (ls *LeakySocket) SetLossRate(lossy bool, rate float32) {
	ls.isLossy = lossy
	ls.lossRate = rate
}

// Close -- close the socket (can also be done on original net.Conn passed to builder)
func (ls *LeakySocket) Close() error {
	return ls.s.Close()
}

// RemoteObjectError is a custom error type used for this library to identify remote methods.
// it is used by both caller and callee endpoints.
type RemoteObjectError struct {
	Err string
}

// getter for the error message included inside the custom error type
func (e *RemoteObjectError) Error() string { return e.Err }

// RequestMsg (this is only a suggestion, can be changed)
//
// RequestMsg represents the request message sent from caller to callee.
// it is used by both endpoints, and uses reflect package to carry
// arbitrary argument types across the network.
type RequestMsg struct {
	Method string
	Args   []reflect.Value
}

// ReplyMsg (this is only a suggestion, can be changed)
//
// ReplyMsg represents the reply message sent from callee back to caller
// in response to a RequestMsg. it similarly uses reflection to carry
// arbitrary return types along with a success indicator to tell the caller
// whether the call was correctly handled by the callee. also includes
// a RemoteObjectError to specify details of any encountered failure.
type ReplyMsg struct {
	Success bool
	Reply   []reflect.Value
	Err     RemoteObjectError
}

// Service -- server side stub/skeleton
//
// A Service encapsulates a multithreaded TCP server that manages a single
// remote object on a single TCP port, which is a simplification to ease management
// of remote objects and interaction with callers.  Each Service is built
// around a single struct of function declarations. All remote calls are
// handled synchronously, meaning the lifetime of a connection is that of a
// single method call.  A Service can encounter a number of different issues,
// and most of them will result in sending a failure response to the caller,
// including a RemoteObjectError with suitable details.
type Service struct {
	// TODO: populate with needed contents including, but not limited to:
	//       - reflect.Type of the Service's interface (struct of Fields)
	//       - reflect.Value of the Service's interface
	//       - reflect.Value of the Service's remote object instance
	//       - status and configuration parameters, as needed
	mu      sync.Mutex
	wg      sync.WaitGroup
	sobj    interface{}
	mIn     map[string][]reflect.Type
	mOut    map[string][]reflect.Type
	port    int
	lossy   bool
	delayed bool
	running bool
	nrCalls int
	ln      net.Listener
}

// NewService -- build a new Service instance around a given struct of supported functions,
// a local instance of a corresponding object that supports these functions,
// and arguments to support creation and use of LeakySocket-wrapped connections.
// performs the following:
// -- returns a local error if function struct or object is nil
// -- returns a local error if any function in the struct is not a remote function
// -- if neither error, creates and populates a Service and returns a pointer
func NewService(ifc interface{}, sobj interface{}, port int, lossy bool, delayed bool) (*Service, error) {
	// NOTE: method function will work on pointers and not structs
	// if interface is nil return error
	if ifc == nil {
		return nil, errors.New("ifc is nil")
	}
	// if object is nil return error
	if sobj == nil {
		return nil, errors.New("sobj is nil")
	}

	// verify if interface and object are pointers to structs
	ifcType := reflect.TypeOf(ifc)
	if ifcType.Kind() != reflect.Pointer && ifcType.Elem().Kind() != reflect.Struct {
		return nil, errors.New("ifc must be a pointer to a struct")
	}

	sobjType := reflect.TypeOf(sobj)
	if sobjType.Kind() != reflect.Pointer && sobjType.Elem().Kind() != reflect.Struct {
		return nil, errors.New("sobj must be a pointer to a struct")
	}

	//
	service := &Service{}

	service.mIn = make(map[string][]reflect.Type)
	service.mOut = make(map[string][]reflect.Type)
	// get interface struct
	ifcStruct := reflect.ValueOf(ifc).Elem()
	// get number of field in struct
	nFields := ifcStruct.NumField()
	// get all methods
	for i := 0; i < nFields; i++ {
		field := ifcStruct.Type().Field(i)
		m := reflect.ValueOf(sobj).MethodByName(field.Name)
		if m.Kind() == reflect.Invalid {
			return nil, errors.New("object must have a method: " + field.Name)
		}

		nOut := reflect.TypeOf(m.Interface()).NumOut()
		// return if method/function is not remote
		if reflect.TypeOf(m.Interface()).Out(nOut-1).String() != "remote.RemoteObjectError" {
			return nil, errors.New("Function " + field.Name + "not a remote function")
		}

		in, out := getMethodDefinitions(reflect.TypeOf(m.Interface()))
		service.mIn[field.Name] = in
		service.mOut[field.Name] = out
	}

	service.port = port
	service.lossy = lossy
	service.delayed = delayed
	service.sobj = sobj
	service.nrCalls = 0

	// if ifc is a pointer to a struct with function declarations,
	// then reflect.TypeOf(ifc).Elem() is the reflected struct's Type

	// if sobj is a pointer to an object instance, then
	// reflect.ValueOf(sobj) is the reflected object's Value

	// TODO: get the Service ready to start
	return service, nil
}

// getMethodDefinitions extracts and returns the input (in) and output (out) argument types
// for a method given its reflection Type. The function takes a method's reflection
// Type as an argument and returns two slices of `reflect.Type` values: one for the
// input argument types and one for the return (output) types.
//
// Parameters:
//   - m (reflect.Type): The reflection Type of the method, which can be obtained
//     using `reflect.TypeOf` or `reflect.ValueOf` on a method.
//
// Returns:
//   - []reflect.Type: A slice containing the types of the input arguments of the method.
//   - []reflect.Type: A slice containing the types of the return values (output) of the method.
//
// Example Usage:
//
//	// Assuming `myMethod` is a method in some struct, and `t` is its reflection Type:
//	in, out := getMethodDefinitions(t)
//	fmt.Println("Input argument types:", in)
//	fmt.Println("Return argument types:", out)
//
// The function loops through the method's input arguments and return types,
// appending each type to separate slices (in for inputs, out for outputs),
// and returns them as two slices.
func getMethodDefinitions(m reflect.Type) ([]reflect.Type, []reflect.Type) {
	// struct fields for the method input
	mIn := m.NumIn()
	var in []reflect.Type
	for nArgs := 0; nArgs < mIn; nArgs++ {
		argType := m.In(nArgs)
		in = append(in, argType)
	}

	mOut := m.NumOut()
	var out []reflect.Type
	for nRet := 0; nRet < mOut; nRet++ {
		retType := m.Out(nRet)
		out = append(out, retType)
	}

	return in, out
}

// Start -- start the Service's tcp listening connection, update the Service
// status, and start receiving caller connections
func (serv *Service) Start() error {
	// TODO: attempt to start a Service created using NewService
	//
	// if called on a service that is already running, print a warning
	// but don't return an error or do anything else
	if serv == nil {
		return nil
	}
	if serv.running {
		return nil
	}
	//
	// otherwise, start the multithreaded tcp server at the given address
	// and update Service state
	//
	// IMPORTANT: Start() should not be a blocking call. once the Service
	// is started, it should return
	//
	//
	// After the Service is started (not to be done inside of this Start
	//      function, but wherever you want):
	//
	// - accept new connections from client callers until someone calls
	//   Stop on this Service, spawning a thread to handle each one
	//
	// - within each client thread, wrap the net.Conn with a LeakySocket
	//   e.g., if Service accepts a client connection `c`, create a new
	//   LeakySocket ls as `ls := LeakySocket(c, ...)`.  then:
	//
	// 1. receive a byte-string on `ls` using `ls.RecvObject()`
	//
	// 2. decoding the byte-string
	//
	// 3. check to see if the service interface's Type includes a method
	//    with the given name
	//
	// 4. invoke method
	//
	// 5. encode the reply message into a byte-string
	//
	// 6. send the byte-string using `ls.SendObject`, noting that the configuration
	//    of the LossySocket does not guarantee that this will work...
	port := ":" + strconv.Itoa(serv.port)
	ln, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatal(err)
	}
	serv.ln = ln
	serv.running = true
	serv.wg.Add(1)
	go serv.serviceWorker()
	return nil
}

// listen to incoming connections and spawn a thread
// to handle remote request
func (serv *Service) serviceWorker() {
	for {
		conn, err := serv.ln.Accept()
		//
		if err != nil {
			serv.wg.Done()
			return
		}

		// handles client request
		go func(c net.Conn) {

			l := NewLeakySocket(c, serv.lossy, serv.delayed)

			var refStruct map[string]interface{}
			buf, e := l.RecvObject()
			for buf == nil {
				// return if socket is closed
				if e.Error() == "RecvObject failed, nil socket" {
					return
				}
				buf, e = l.RecvObject()
			}

			e = json.Unmarshal(buf, &refStruct)

			if e != nil {
				log.Fatal("Service Worker Unmarshall", e.Error())
			}

			var outs []reflect.Value

			methodName := reflect.ValueOf(refStruct["name"]).String()

			mIn, ok := serv.mIn[methodName]
			mOut := serv.mOut[methodName]
			// Send error and return if method/param mismatch
			if !ok {
				outs = replyErrMsgs(1, reflect.TypeOf(nil), "Method not found!")
				sendMsgs(l, outs)
				return
			}

			// The below code performs the following steps:
			// 1. Extracts the "Params" field from refStruct and casts it to a map[string]interface{}.
			// 2. Initializes a slice to hold the reflect.Values for the method parameters.
			// 3. Iterates over the expected parameters defined in mIn:
			//    - Retrieves each corresponding value from inVals using a dynamic key format.
			//    - Determines the expected type from mIn and processes the value accordingly.
			// 4. Supports various types including:
			//    - Primitive types: string, int, bool, float64
			//    - Structs: processed using procStruct to create the appropriate reflect.Value
			//    - Slices: constructed by appending processed struct values to a reflect.Value slice.
			// 5. Each processed reflect.Value is appended to the params slice for the method call.
			//
			//listParam := refStruct["Params"].(map[string]interface{})
			retParam := refStruct["Outs"].(map[string]interface{})
			inVals := refStruct["Params"].(map[string]interface{})
			numIn := len(mIn)

			var params []reflect.Value
			var p reflect.Value

			if len(mIn) != len(inVals) {
				outs = replyErrMsgs(1, reflect.TypeOf(nil), "Incorrect number of return arguments!")
				sendMsgs(l, outs)
				return
			}

			var paramMatch = true
			for i := 0; i < numIn; i++ {
				v := inVals["Field"+strconv.Itoa(i+1)]
				k := mIn[i].Kind().String()
				paramMatch = assertType(k, v)
				if !paramMatch {
					break
				}
			}

			if !paramMatch {
				outs = replyErrMsgs(1, reflect.TypeOf(nil), "Parameter Mismatch!")
				sendMsgs(l, outs)
				return
			}

			if len(mOut) != len(retParam) {
				outs = replyErrMsgs(1, reflect.TypeOf(nil), "Incorrect number of return arguments!")
				sendMsgs(l, outs)
				return
			}

			for i := 0; i < numIn; i++ {
				v := inVals["Field"+strconv.Itoa(i+1)]
				k := mIn[i].Kind()
				p = mapToGoType(k, mIn[i], v)
				params = append(params, reflect.ValueOf(p.Interface()))
			}

			// run method
			val := reflect.ValueOf(serv.sobj).MethodByName(methodName).Call(params)
			//count := len(val) - 1
			count := len(val)
			var value reflect.Value
			for i := 0; i < count; i++ {
				if val[i].Kind() == reflect.Map || val[i].Kind() == reflect.Slice || val[i].Kind() == reflect.Struct {
					value = processCompositeReturns(val[i].Kind(), val[i].Type(), val[i])
				} else {
					value = val[i]
				}
				outs = append(outs, value)
			}

			res := createResObject(outs)
			// encode/serialize method request to remote service
			byteS, _ := json.Marshal(res.Interface())
			// send response
			b, sendErr := l.SendObject(byteS)
			for !b {
				// return if socket is closed
				if sendErr != nil {
					if strings.Contains(sendErr.Error(), "nil socket") {
						return
					}
				}
				b, sendErr = l.SendObject(byteS)
			}

			serv.mu.Lock()
			serv.nrCalls++
			serv.mu.Unlock()

			_ = conn.Close()

			return
		}(conn)
	}
}

// createResObject constructs a new struct type dynamically based on the provided
// slice of reflect.Values. Each value in the input slice is used to define a field
// in the resulting struct, which will be sent over a remote network.
//
// The function performs the following steps:
//  1. Initializes a slice to hold the struct fields.
//  2. Iterates over the input slice of reflect.Values, creating a new struct field
//     for each value with a name format of "ReturnX", where X is the field index.
//  3. Defines a new struct type using reflect.StructOf with the created fields.
//  4. Instantiates a new struct object of the defined type.
//  5. Sets the fields of the newly created struct object to the corresponding values
//     from the input slice.
//
// Parameters:
//   - in: A slice of reflect.Values representing the values to be included as fields
//     in the resulting struct.
//
// Returns:
//   - reflect.Value: A reflect.Value representing the newly created struct object
//     populated with the input values.
//
// Example Usage:
//
//	values := []reflect.Value{
//	    reflect.ValueOf("Success"),
//	    reflect.ValueOf(42),
//	}
//	resObj := createResObject(values)
//	fmt.Println(resObj.Interface()) // Outputs the constructed struct
func createResObject(in []reflect.Value) reflect.Value {
	var sFields []reflect.StructField
	//
	n := len(in)
	for nArgs := 0; nArgs < n; nArgs++ {
		argType := in[nArgs].Interface()
		s := reflect.StructField{
			Name: "Return" + strconv.Itoa(nArgs+1),
			Type: reflect.TypeOf(argType),
		}
		sFields = append(sFields, s)
	}
	// create struct
	msgStruct := reflect.StructOf(sFields)
	// create a struct object based on reflect struct definitions
	resObj := reflect.ValueOf(reflect.New(msgStruct).Interface())
	//
	for nArgs := 0; nArgs < n; nArgs++ {
		v := in[nArgs]
		resObj.Elem().Field(nArgs).Set(v)
	}

	return resObj
}

func (serv *Service) GetCount() int {
	// TODO: return the total number of remote calls served successfully by this Service
	serv.mu.Lock()
	defer serv.mu.Unlock()
	return serv.nrCalls
}

func (serv *Service) IsRunning() bool {
	// TODO: return a boolean value indicating whether the Service is running
	return serv.running
}

func (serv *Service) Stop() {
	// TODO: stop the Service, change state accordingly, clean up any resources

	//serv.mu.Lock()
	if serv.running {
		err := serv.ln.Close()
		serv.wg.Wait()
		serv.ln = nil
		if err != nil {
			log.Fatal(err)
		}
		serv.running = false

	}
	//serv.mu.Unlock()

}

// holds the client/StubFactory info about the remote service location
type stubInfo struct {
	adr     string
	lossy   bool
	delayed bool
}

// StubFactory -- make a client-side stub
//
// StubFactory uses reflection to populate the interface functions to create the
// caller's stub interface. Only works if all functions are exported/public.
// Once created, the interface masks remote calls to a Service that hosts the
// object instance that the functions are invoked on.  The network address of the
// remote Service must be provided with the stub is created, and it may not change later.
// A call to StubFactory requires the following inputs:
// -- a struct of function declarations to act as the stub's interface/proxy
// -- the remote address of the Service as "<ip-address>:<port-number>"
// -- indicator of whether caller-to-callee channel has emulated packet loss
// -- indicator of whether caller-to-callee channel has emulated propagation delay
// performs the following:
// -- returns a local error if function struct is nil
// -- returns a local error if any function in the struct is not a remote function
// -- otherwise, uses reflection to access the functions in the given struct and
//
//	populate their function definitions with the required stub functionality
func StubFactory(ifc interface{}, adr string, lossy bool, delayed bool) error {
	// if ifc is a pointer to a struct with function declarations,
	// then reflect.TypeOf(ifc).Elem() is the reflected struct's reflect.Type
	// and reflect.ValueOf(ifc).Elem() is the reflected struct's reflect.Value
	//
	// Here's what it needs to do (not strictly in this order):
	//
	//    1. create a request message populated with the method name and input
	//       arguments to send to the Service
	//
	//    2. create a []reflect.Value of correct size to hold the result to be
	//       returned back to the program
	//
	//    3. connect to the Service's tcp server, and wrap the connection in an
	//       appropriate LeakySocket using the parameters given to the StubFactory
	//
	//    4. encode the request message into a byte-string to send over the connection
	//
	//    5. send the encoded message, noting that the LeakySocket is not guaranteed
	//       to succeed depending on the given parameters
	//
	//    6. wait for a reply to be received using RecvObject, which is blocking
	//        -- if RecvObject returns an error, populate and return error output
	//
	//    7. decode the received byte-string according to the expected return types

	// return error if interface/struct is null
	if ifc == nil {
		return errors.New("ifc is nil")
	}

	// return error if interface/struct is null
	if reflect.TypeOf(ifc).Kind() != reflect.Ptr {
		return errors.New("ifc needs to be a pointer to a struct")
	}

	// return error if pointer does not point to a struct
	if reflect.TypeOf(ifc).Elem().Kind() != reflect.Struct {
		return errors.New("ifc pointer does not point to a struct")
	}

	// remote server address and network propagation
	sI := stubInfo{
		adr:     adr,
		lossy:   lossy,
		delayed: delayed,
	}

	// get actual interface
	// get all fields in struct
	// if field is a function
	// get function name
	// get function type's i'th input parameter
	// create struct with the method and inputs of the function
	// send struct through leaky socket to remote service
	ifcStruct := reflect.ValueOf(ifc).Elem()
	numFields := ifcStruct.NumField()
	for i := 0; i < numFields; i++ {
		fName := ifcStruct.Type().Field(i).Name
		field := ifcStruct.Field(i)
		if field.Kind() == reflect.Func {
			funcF := field.Type()
			nOut := funcF.NumOut()
			// return if method/function is not remote
			if funcF.Out(nOut-1).String() != "remote.RemoteObjectError" {
				return errors.New("Function " + fName + "not a remote function")
			}

			// function to send request to remote service
			remoteCall := func(in []reflect.Value) (results []reflect.Value) {
				// create reflect field for struct
				// NOTE: struct tag. specifies which filed should be present in a struct
				// if tag is specified, other struct about to use the values should
				// also specify the struct, or values will not be created
				var sFields []reflect.StructField

				// struct fields for the method input
				n := len(in)
				var params []reflect.Value
				var param reflect.Value
				for nArgs := 0; nArgs < n; nArgs++ {
					//argType := in[nArgs].Interface()
					k := in[nArgs].Kind()
					switch k {
					case reflect.Map:
						kt := in[nArgs].Type().Key()
						vt := in[nArgs].Type().Elem()
						if kt.Kind() != reflect.String {
							outs := replyErrMsgs(nOut, funcF, "Map key type is currently not supported!")
							return outs
						}
						if vt.Kind() == reflect.Struct {
							param = reflect.ValueOf(make(map[string]interface{}))
							for _, key := range in[nArgs].MapKeys() {
								// Get the value associated with the key
								val := in[nArgs].MapIndex(key)
								mvs := CreateAnnotatedStruct(val)
								param.SetMapIndex(key, mvs)
							}
						} else if vt.Kind() == reflect.Slice {
							elemType := vt.Elem()
							if elemType.Kind() == reflect.Struct {
								param = reflect.ValueOf(make(map[string]interface{}))
								for _, key := range in[nArgs].MapKeys() {
									// Get the value associated with the key
									val := in[nArgs].MapIndex(key)
									length := val.Len()
									// Define a slice type of interface{}
									sliceType := reflect.TypeOf([]interface{}{})
									//p := reflect.MakeSlice(elemType, 0, 0)
									p := reflect.MakeSlice(sliceType, 0, 0)
									for num := 0; num < length; num++ {
										sliceValue := CreateAnnotatedStruct(val.Index(num))
										p = reflect.Append(p, sliceValue)
									}
									param.SetMapIndex(key, p)
								}

							}

						} else {
							param = in[nArgs]
						}
						params = append(params, param)
					case reflect.Struct:
						param = CreateAnnotatedStruct(in[nArgs])
						params = append(params, param)
					case reflect.Slice:
						if in[nArgs].Len() == 0 {
							param = in[nArgs]
							params = append(params, param)
							break
						}

						elemType := in[nArgs].Index(0)
						length := in[nArgs].Len()
						if elemType.Kind() == reflect.Struct {
							st := CreateAnnotatedStruct(in[nArgs].Index(0))
							// Create a slice type
							sliceType := reflect.SliceOf(reflect.TypeOf(st.Interface()))
							param = reflect.MakeSlice(sliceType, length, length)
							for num := 0; num < length; num++ {
								sliceValue := CreateAnnotatedStruct(in[nArgs].Index(num))
								param.Index(num).Set(reflect.ValueOf(sliceValue.Interface()))
							}
						}

						params = append(params, reflect.ValueOf(param.Interface()))

					default:
						param = in[nArgs]
						params = append(params, param)
					}

					s := reflect.StructField{
						Name: "Field" + strconv.Itoa(nArgs+1),
						Type: reflect.TypeOf(param.Interface()),
						//Tag:  `json:"input"`,
					}
					sFields = append(sFields, s)

				}

				// define a struct with the specified field
				msgStruct := reflect.StructOf(sFields)

				// create a struct object with the arguments to send to remote server
				reqObj := reflect.ValueOf(reflect.New(msgStruct).Interface())
				// assign values for struct
				for nArgs := 0; nArgs < n; nArgs++ {
					v := params[nArgs]
					reqObj.Elem().Field(nArgs).Set(reflect.ValueOf(v.Interface()))
				}

				// create struct fields of expected return
				var rFields []reflect.StructField
				for nRet := 0; nRet < nOut-1; nRet++ {
					typ := funcF.Out(nRet)
					argType := reflect.New(typ).Elem().Type()
					rF := reflect.StructField{
						Name: "Return" + strconv.Itoa(nRet+1),
						Type: argType,
						//Tag:  `json:"input"`,
					}
					rFields = append(rFields, rF)
				}
				// create struct fields of expected return
				remObj := RemoteObjectError{}
				rF := reflect.StructField{
					Name: "Return" + strconv.Itoa(nOut),
					Type: reflect.TypeOf(remObj),
					//Tag:  `json:"input"`,
				}
				rFields = append(rFields, rF)

				// create a struct object with the  expected return type and # to send to remote server
				retStruct := reflect.StructOf(rFields)
				retObj := reflect.ValueOf(reflect.New(retStruct).Interface())

				// struct fields to hold the argument and expected return to send to remote server
				var wFields []reflect.StructField
				nF := reflect.StructField{
					Name: fName,
					Type: reflect.TypeOf(fName),
					Tag:  `json:"name"`,
				}
				wFields = append(wFields, nF)

				firstF := reflect.StructField{
					Name: "Params",
					Type: reflect.TypeOf(reqObj.Elem().Interface()),
					//Tag:  `json:"input"`,
				}
				wFields = append(wFields, firstF)

				secondF := reflect.StructField{
					Name: "Outs",
					Type: reflect.TypeOf(retObj.Elem().Interface()),
					//Tag:  `json:"input"`,
				}
				wFields = append(wFields, secondF)

				// define a struct to send to remote server
				inOutStruct := reflect.StructOf(wFields)

				// create a struct object based on reflect struct definitions
				reqServerObj := reflect.ValueOf(reflect.New(inOutStruct).Interface())
				reqServerObj.Elem().Field(0).SetString(fName)
				// assign values for struct
				v := reflect.ValueOf(reqObj.Elem().Interface())
				reqServerObj.Elem().Field(1).Set(v)

				// encode/serialize method request to remote service
				byteS, err := json.Marshal(reqServerObj.Elem().Interface())
				if err != nil {
					panic(err)
				}

				// connect to remote service
				// panic if error encountered
				c, err := net.Dial("tcp", sI.adr)
				if err != nil {
					if strings.Contains(err.Error(), "connect: connection refused") {
						//outs := connectionRefused(nOut, funcF)
						outs := replyErrMsgs(nOut, funcF, "nil socket")
						return outs
					}
				}

				// new leaky socket object
				l := NewLeakySocket(c, sI.lossy, sI.delayed)

				// send method request
				// resend if method cannot be sent
				b, err := l.SendObject(byteS)
				for !b {
					// return if socket is closed
					if err != nil {
						if strings.Contains(err.Error(), "nil socket") {
							outs := replyErrMsgs(nOut, funcF, "nil socket")
							return outs
						}
					}
					b, err = l.SendObject(byteS)
				}

				// receive results from remote service
				byteR, err := l.RecvObject()
				for byteR == nil {
					// return if socket is closed
					if strings.Contains(err.Error(), "nil socket") {
						//outs := connectionRefused(nOut, funcF)
						outs := replyErrMsgs(nOut, funcF, "nil socket")
						return outs
					}
					byteR, err = l.RecvObject()
				}

				var respData map[string]interface{}
				er := json.Unmarshal(byteR, &respData)
				if er != nil {
					panic(er)
				}

				nRet := len(respData)
				if nRet == 1 || nRet != nOut {
					var outs []reflect.Value
					ifcVal := respData["Return"+strconv.Itoa(+1)].(map[string]interface{})
					val, ok := ifcVal["Err"].(string)
					if !ok {
						val = ifcVal["Err_string_1"].(string)
					}
					outs = replyErrMsgs(nOut, funcF, val)
					return outs
				}

				//respData := reflect.ValueOf(jsonRlt)
				var outs []reflect.Value
				var out reflect.Value
				for ct := 0; ct < nOut; ct++ {
					val := respData["Return"+strconv.Itoa(ct+1)]
					t := funcF.Out(ct)
					k := t.Kind()
					out = mapToGoType(k, t, val)
					outs = append(outs, out)

				}
				return outs
			}

			// fptr is a pointer to a function.
			fptr := ifcStruct.Field(i)

			// Make a function of the right type.
			v := reflect.MakeFunc(fptr.Type(), remoteCall)

			// Assign it to the value fn represents.
			fptr.Set(v)
		}
	}

	// nil if stub factory was created
	return nil
}

// replyErrMsgs constructs and returns a slice of reflect.Value objects representing
// the output values of a function. The function is typically used when a function
// is expected to return multiple values, and one of those values should be an error message
// or a custom error object. The function fills all output values with default values
// based on their types, and appends a custom error object as the last return value.
//
// Parameters:
//   - nOut (int): The number of output (return) values that the function is expected to return.
//   - funcF (reflect.Type): The reflection Type of the function that defines the return types.
//   - errMsg (string): The error message to be encapsulated into a custom error object.
//
// Returns:
//   - []reflect.Value: A slice of reflect.Value representing the default values for
//     all output arguments of the function, with the last value being a custom error object.
//
// Example Usage:
//
//	// Assuming a function signature like `func SomeFunction() (string, int, error)`:
//	outs := replyErrMsgs(3, reflect.TypeOf(SomeFunction), "An error occurred")
//	fmt.Println(outs[0].Interface())  // Empty string (default value)
//	fmt.Println(outs[1].Interface())  // 0 (default value)
//	fmt.Println(outs[2].Interface())  // RemoteObjectError{Error: "An error occurred"}
//
// The function first creates a slice `outs` to store the return values. It then initializes
// each return value to its zero value based on the types specified in `funcF`. The last value
// in the slice is set to the `RemoteObjectError` with the provided error message.
func replyErrMsgs(nOut int, funcF reflect.Type, errMsg string) []reflect.Value {
	var outs []reflect.Value
	rmObj := RemoteObjectError{errMsg}
	for nArgs := 0; nArgs < nOut-1; nArgs++ {
		t := funcF.Out(nArgs)
		val := reflect.New(t).Elem()
		//t := reflect.ValueOf(val).Elem().Interface()
		outs = append(outs, val)
	}
	outs = append(outs, reflect.ValueOf(rmObj))
	return outs
}

// processCompositeData constructs a new struct type based on the fields of the
// provided reflect.Value input. It processes the fields of the input struct, creating
// a new struct type with field names that include the original field names, their
// types, and their positions in the original struct.
//
// The resulting struct is populated with the corresponding values from the input
// reflect.Value and returned as a reflect.Value.
//
// Parameters:
//   - v: A reflect.Value representing the input struct whose fields are to be
//     processed. This value must be a struct.
//
// Returns:
//   - reflect.Value: A reflect.Value representing the newly created struct populated
//     with values from the input struct.
//
// Example Usage:
//
//	type Composite struct {
//	    ID   int
//	    Name string
//	}
//	comp := Composite{ID: 1, Name: "Alice"}
//	in := reflect.ValueOf(comp)
//	result := processCompositeData(in)
//	fmt.Println(result.Interface()) // Outputs the constructed struct
func processCompositeData(v reflect.Value) reflect.Value {

	num := v.NumField()
	var sFields []reflect.StructField
	// struct fields for the method input
	for n := 0; n < num; n++ {
		nm := v.Type().Field(n).Name
		f := v.Field(n)
		i := f.Interface()
		t := reflect.TypeOf(i)
		s := reflect.StructField{
			Name: nm + "_" + t.Kind().String() + "_" + strconv.Itoa(n+1),
			Type: t,
		}
		sFields = append(sFields, s)
	}

	// define a struct with the specified field
	st := reflect.StructOf(sFields)
	// create a struct object with the arguments to send to remote server
	data := reflect.ValueOf(reflect.New(st).Interface())

	for i := 0; i < num; i++ {
		f := v.Field(i)
		data.Elem().Field(i).Set(f)
	}

	return data
}

// procStruct converts a map of records into a dynamically constructed struct.
// Each entry in the map should follow a specific naming convention:
// "FieldName_Type_Index", where:
// - FieldName is the name of the field in the resulting struct.
// - Type indicates the Go type of the field (e.g., "string", "int").
// - Index indicates the position of the field in the struct (1-based index).
//
// The function initializes a new struct type with the fields defined in the
// input map, populates it with values, and returns a reflect.Value
// representing the constructed struct.
//
// Parameters:
//   - records: A map[string]interface{} containing field definitions and values.
//     Each key must conform to the expected format, and the corresponding value
//     should be compatible with the specified type.
//
// Returns:
//   - reflect.Value: A reflect.Value representing the dynamically created struct
//     populated with the provided values.
//
// Example Usage:
//
//	records := map[string]interface{}{
//	    "Name_string_1": "Alice",
//	    "Age_int_2":     30,
//	    "Active_bool_3": true,
//	}
//
//	result := procStruct(records)
//	fmt.Println(result.Interface()) // Outputs the constructed struct
func procStruct(records map[string]interface{}) reflect.Value {
	num := len(records)
	sFields := make([]reflect.StructField, num)
	values := make([]reflect.Value, num)
	for key, val := range records {
		list := strings.Split(key, "_")
		if len(list) != 3 {
			log.Fatal("Invalid key format")
		}

		name := list[0]
		typ := list[1]
		n, err := strconv.Atoi(list[2])
		if err != nil {
			log.Fatal(err)
		}

		t, v := getTypeAndValue(typ, val)
		sFields[n-1] = reflect.StructField{Name: name, Type: t}

		values[n-1] = v
	}

	// define a struct with the specified field
	st := reflect.StructOf(sFields)
	// create a struct object with the arguments to send to remote server
	data := reflect.ValueOf(reflect.New(st).Interface())
	for i := 0; i < num; i++ {
		f := values[i]
		data.Elem().Field(i).Set(f)
	}

	return data.Elem()
}

// getTypeAndValue determines the Go type and value for a given input based on
// a specified type string. The function expects the type string to indicate
// the desired Go type (e.g., "string", "int", "bool", "float64") and returns
// the corresponding reflect.Type and reflect.Value.
//
// Parameters:
//   - typ: A string representing the type of the input value. Valid options are
//     "string", "int", "bool", and "float64".
//   - ifc: An interface{} representing the input value. The actual type of this
//     value must match the specified typ for correct type assertion.
//
// Returns:
//   - reflect.Type: The reflect.Type corresponding to the specified typ.
//   - reflect.Value: The reflect.Value representing the input value, converted
//     to the appropriate type.
//
// Example Usage:
//
//	value := 42.0
//	typ := "int"
//	t, v := getTypeAndValue(typ, value)
//	fmt.Println(t) // Outputs: int
//	fmt.Println(v.Int()) // Outputs: 42
func getTypeAndValue(typ string, ifc interface{}) (reflect.Type, reflect.Value) {
	var v reflect.Value
	var t reflect.Type
	var data interface{}
	switch typ {
	case "string":
		data = ifc.(string)

	case "int":
		data = int(ifc.(float64))
	case "bool":
		data = ifc.(bool)
	case "float64":
		data = ifc.(float64)

	}

	t = reflect.TypeOf(data)
	v = reflect.ValueOf(data)
	return t, v
}

// CreateAnnotatedStruct constructs a new struct type based on the fields of the provided
// reflect.Value input. It retrieves the fields of the input struct and dynamically
// creates a new struct type with field names that include the original field names,
// their types, and their positions in the original struct.
//
// The resulting struct is populated with the corresponding values from the input
// reflect.Value and returned as a reflect.Value.
//
// Parameters:
//   - in: A reflect.Value representing the input struct whose fields are to be
//     processed. This value must be a struct.
//
// Returns:
//   - reflect.Value: A reflect.Value representing the newly created struct populated
//     with values from the input struct.
//
// Example Usage:
//
//	type Request struct {
//	    ID   int
//	    Name string
//	}
//	req := Request{ID: 1, Name: "Alice"}
//	in := reflect.ValueOf(req)
//	result := procReqStruct(in)
//	fmt.Println(result.Interface()) // Outputs the constructed struct
func CreateAnnotatedStruct(in reflect.Value) reflect.Value {
	num := in.NumField()
	sFields := make([]reflect.StructField, num)
	values := make([]reflect.Value, num)
	for i := 0; i < num; i++ {
		name := in.Type().Field(i).Name
		v := in.Field(i)

		t := reflect.TypeOf(v.Interface())
		s := reflect.StructField{
			Name: name + "_" + t.Kind().String() + "_" + strconv.Itoa(i+1),
			//Name: name,
			Type: t,
		}
		sFields[i] = s
		values[i] = v
	}

	// define a struct with the specified field
	st := reflect.StructOf(sFields)
	// create a struct object with the arguments to send to remote server
	data := reflect.ValueOf(reflect.New(st).Interface())
	for i := 0; i < num; i++ {
		f := values[i]
		data.Elem().Field(i).Set(f)
	}

	return data.Elem()
}

// mapToGoType converts an interface value into a specific Go type based on
// the provided kind and type information using reflection.
//
// Parameters:
//   - kind: The kind of the type (e.g., Struct, Slice, Map).
//   - typ: The reflect.Type representing the desired Go type.
//   - v: The value to be converted, typically an interface{} that holds
//     data in the form of map[string]interface{} or slices.
//
// Returns:
// - A reflect.Value containing the converted value of the specified type.
//
// The function processes the input value according to its kind,
// constructing the appropriate Go type dynamically.
func mapToGoType(kind reflect.Kind, typ reflect.Type, v interface{}) reflect.Value {
	var p reflect.Value
	switch kind {
	case reflect.Struct:
		p = procStruct(v.(map[string]interface{}))
	case reflect.Slice:
		arr := v.([]interface{})
		if len(arr) == 0 {
			p = reflect.MakeSlice(typ, 0, 0)
			return p
		}
		elemType := typ.Elem()

		length := len(arr)
		p = reflect.MakeSlice(typ, 0, 0)
		if elemType.Kind() == reflect.Struct {
			for num := 0; num < length; num++ {
				sliceValue := procStruct(arr[num].(map[string]interface{}))
				p = reflect.Append(p, sliceValue)
			}
		} else {
			t := elemType.Kind().String()
			for num := 0; num < length; num++ {
				_, sliceValue := getTypeAndValue(t, arr[num])
				p = reflect.Append(p, sliceValue)
			}
		}
	case reflect.Map:
		mv := reflect.ValueOf(v.(interface{}))
		miv := v.(map[string]interface{})
		p = reflect.MakeMap(typ)
		//kt := mIn[i].Key()
		vt := typ.Elem()
		elemType := vt.Elem()
		if vt.Kind() == reflect.Struct {
			for _, key := range mv.MapKeys() {
				ifcVal := miv[key.String()]
				mvs := procStruct(ifcVal.(map[string]interface{}))
				p.SetMapIndex(key, mvs)
			}
		} else if vt.Kind() == reflect.Slice {
			if elemType.Kind() == reflect.Struct {
				for _, key := range mv.MapKeys() {
					ifcVal := miv[key.String()]
					//val := mv.MapIndex(key)
					arrVal := ifcVal.([]interface{})
					length := len(arrVal)
					arr := reflect.MakeSlice(vt, 0, 0)
					for num := 0; num < length; num++ {
						sliceValue := procStruct(arrVal[num].(map[string]interface{}))
						arr = reflect.Append(arr, sliceValue)
					}
					p.SetMapIndex(key, arr)
				}
			} else {
				for _, key := range mv.MapKeys() {
					ifcVal := miv[key.String()]
					arrVal := ifcVal.([]interface{})
					length := len(arrVal)
					arr := reflect.MakeSlice(vt, 0, 0)
					for num := 0; num < length; num++ {
						_, sliceValue := getTypeAndValue(elemType.Kind().String(), arrVal[num])
						arr = reflect.Append(arr, sliceValue)
					}
					p.SetMapIndex(key, arr)
				}
			}
		} else {
			for _, key := range mv.MapKeys() {
				ifcVal := miv[key.String()]
				_, val := getTypeAndValue(elemType.Kind().String(), ifcVal)
				p.SetMapIndex(key, val)
			}
		}

	default:
		_, p = getTypeAndValue(kind.String(), v)

	}

	return p
}

// assertType checks if the provided interface{} can be asserted to the specified type.
// It returns true if the assertion is successful and false if it fails or panics.
//
// Parameters:
// - typ: A string representing the expected type ("string", "int", "bool", "float64").
// - ifc: The interface{} value to be checked.
//
// Returns:
// - A boolean indicating whether the type assertion was successful or not.
//
// This function uses a deferred function to recover from panics during type assertions.
func assertType(typ string, ifc interface{}) bool {

	result := true
	defer func() {
		if r := recover(); r != nil {
			result = false
		}
	}()

	switch typ {
	case "string":
		_ = ifc.(string)

	case "int":
		_ = int(ifc.(float64))
	case "bool":
		_ = ifc.(bool)
	case "float64":
		_ = ifc.(float64)

	}

	return result
}

// sendMsgs serializes and sends a message over a LeakySocket to a remote service.
//
// The function marshals a slice of output values into a response object, serializes it
// to a JSON byte slice, and sends the byte slice using the provided LeakySocket.
// It retries sending the message if the socket is not ready, and stops if the socket is closed.
//
// Parameters:
//   - l (*LeakySocket): The LeakySocket instance used to send the message.
//   - outs ([]reflect.Value): A slice of reflect.Value elements to be serialized and sent.
//
// Behavior:
//   - Creates a response object from the `outs` slice.
//   - Serializes the response object to a JSON byte slice.
//   - Sends the byte slice over the provided LeakySocket.
//   - Retries sending the message if the socket is not yet ready or if there was an error,
//     unless the socket is closed (determined by checking the error message).
func sendMsgs(l *LeakySocket, outs []reflect.Value) {
	resObj := createResObject(outs)
	// encode/serialize method request to remote service
	byteS, _ := json.Marshal(resObj.Interface())
	// send response
	b, sendErr := l.SendObject(byteS)
	for !b {
		// return if socket is closed
		if sendErr != nil {
			if strings.Contains(sendErr.Error(), "nil socket") {
				return
			}
		}
		b, sendErr = l.SendObject(byteS)
	}
}

// processCompositeReturns processes composite Go types (structs, slices, and maps)
// using reflection, applying specific transformations based on the kind of the value.
// It returns a processed value based on the type of the input.
//
// Parameters:
//   - kind (reflect.Kind): The kind of the value (e.g., reflect.Struct, reflect.Slice, reflect.Map).
//   - typ (reflect.Type): The type of the value, used for determining element types for slices and maps.
//   - val (reflect.Value): The value to be processed. This can be a struct, slice, or map.
//
// Returns:
//   - reflect.Value: The processed value, which could be a transformed struct, slice, or map.
func processCompositeReturns(kind reflect.Kind, typ reflect.Type, val reflect.Value) reflect.Value {
	var p reflect.Value
	switch kind {
	case reflect.Struct:
		p = processCompositeData(val)
	case reflect.Slice:
		if val.Len() == 0 {
			return val
		}
		elemType := typ.Elem()
		length := val.Len()
		sliceType := reflect.TypeOf([]interface{}{})
		p = reflect.MakeSlice(sliceType, 0, 0)
		if elemType.Kind() == reflect.Struct {
			for num := 0; num < length; num++ {
				sliceValue := processCompositeData(val.Index(num))
				p = reflect.Append(p, sliceValue)
			}
		} else {
			t := elemType.Kind().String()
			for num := 0; num < length; num++ {
				_, sliceValue := getTypeAndValue(t, val.Index(num).Interface())
				p = reflect.Append(p, sliceValue)
			}
		}
	case reflect.Map:
		p = reflect.ValueOf(make(map[string]interface{}))
		vt := typ.Elem()
		elemType := vt.Elem()
		if vt.Kind() == reflect.Struct {
			for _, key := range val.MapKeys() {
				mvs := processCompositeData(val.MapIndex(key))
				p.SetMapIndex(key, mvs)
			}
		} else if vt.Kind() == reflect.Slice {
			if elemType.Kind() == reflect.Struct {
				for _, key := range val.MapKeys() {
					arrVal := val.MapIndex(key)
					length := arrVal.Len()
					sliceType := reflect.TypeOf([]interface{}{})
					arr := reflect.MakeSlice(sliceType, 0, 0)
					for num := 0; num < length; num++ {
						sliceValue := processCompositeData(arrVal.Index(num))
						arr = reflect.Append(arr, sliceValue)
					}
					p.SetMapIndex(key, arr)
				}
			} else {
				for _, key := range val.MapKeys() {
					arrVal := val.MapIndex(key)
					length := arrVal.Len()
					arr := reflect.MakeSlice(vt, 0, 0)
					for num := 0; num < length; num++ {
						_, sliceValue := getTypeAndValue(elemType.Kind().String(), arrVal.Index(num).Interface())
						arr = reflect.Append(arr, sliceValue)
					}
					p.SetMapIndex(key, arr)
				}
			}
		} else {
			for _, key := range val.MapKeys() {
				ifcVal := val.MapIndex(key)
				_, val = getTypeAndValue(elemType.Kind().String(), ifcVal.Interface())
				p.SetMapIndex(key, val)
			}
		}

	}

	return p
}
