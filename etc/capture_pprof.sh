curl -s -o metrics localhost:8082/metrics
curl -s -o cmdline localhost:8082/debug/pprof/cmdline
curl -s -o symbol localhost:8082/debug/pprof/symbol
curl -s -o trace localhost:8082/debug/pprof/trace
curl -s -o block localhost:8082/debug/pprof/block
curl -s -o goroutine localhost:8082/debug/pprof/goroutine
curl -s -o heap localhost:8082/debug/pprof/heap
curl -s -o threadcreate localhost:8082/debug/pprof/threadcreate
echo captured everything but profile - please be patient about 30sec
date
curl -s -o profile localhost:8082/debug/pprof/profile
date
