package main

import (
	"context"
	"log"
	"runtime"

	"pusher/internal"
)

func init() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	/**** PROFILING - REMOVE BEFORE PRODUCTION ****/
	// also imported _ "net/http/pprof"
	// go func() {
	// 	log.Logger().Println("Starting pprof on :6060")
	// 	log.Logger().Println(http.ListenAndServe("localhost:6060", nil))
	// }()
	/**** END PROFILING ****/
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := internal.Run(ctx); err != nil {
		log.Fatalf("run server failed: %v", err)
	}
}
