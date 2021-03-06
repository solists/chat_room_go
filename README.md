# chat_room_go
 Simple implementation of chat website written in Golang
 EXCEPTIONALY not the best realisation. GRPC used for demo purposes, same for databses, there are at least 3 databases for such a small project.
 Things that may be done better, excluding functionality:
 - fasthttp over default http
 - provide docker compose, add dockerfile for this app, not only for databases
 - use web storage, not cookies
 - add tests
 - create custom errors
 - add persist db config for dockered databases
 - https, let`s encrypt, use tls not only in grpc
 - websockets, pool of workers
 - documentation
 - monitoring by timer
 - more panic recovery
 - less databases, no grpc
 - another module structure, several repos or one common module

 # Installation
 - Run all .sh files inside db directory, it will run all databases, that app needs, redis and mongo
 - Clickhouse is not currently essential, to make it work, run build and run for corresponding Dockerfile, and change default logger (currently it writes to directory) inside utils/logs/log.go init() function, see commented lines
 - Run install.sh in the root folder, it will run app in the container, generate keys for tls proto communication, run all microservices

# Dev log
## V01
Naive realization of chat program.
Alongside the used global structures and unscalable architecture it do consist some other bugs:
Race condition (handle funcs are concurrent, structures are indeed global), questionable way of rendering frontend (gets data from static http file)
There is no auth in main handler frontend, so if session expires it continues to receive updates (even will be able to push message once)

## V02
Structures are the same, so the problems. Significanly refactured frontend logic. Changed some backend logic.

## V03
Added clickhouse microservice realization via grpc. Added zap logger client for this microservice, so all logs now go to clickhouse.

## V04
Added simple mongodb microservice via grpc. Mongo stores all messages.

## V05
Added simple redis microservice via grpc. Redis stores all users.

## Cleanup
Refacturing. Added http log, panic recovery, auth middleware. Prepared for delivery, added dockerfiles and scripts.

