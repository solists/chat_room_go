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
 - https, let` encrypt, use tls not only in grpc
 - websockets, pool of workers
 - documentation
 - monitoring by timer
 - more panic recovery
 - less databases, no grpc


# V01
Naive realization of chat program.
Alongside the used global structures and unscalable architecture it do consist some other bugs:
Race condition (handle funcs are concurrent, structures are indeed global), questionable way of rendering frontend (gets data from static http file)
There is no auth in main handler frontend, so if session expires it continues to receive updates (even will be able to push message once) 5h

# V02
Structures are the same, so the problems. Significanly refactured frontend logic. Changed some backend logic. 4h

# V03
Added clickhouse microservice realization via grpc. Added zap logger client for this microservice, so all logs now go to clickhouse. 12h

# V04
Added simple mongodb microservice via grpc. Mongo stores all messages. 2h

# V05
Added simple redis microservice via grpc. Redis stores all users. 2h

