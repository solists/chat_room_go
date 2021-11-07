
docker build -f microservices/redis/Dockerfile -t redis-micro:v1 . 
docker run -d --rm -p 8083:8083 redis-micro:v1

docker build -f microservices/mongodb/Dockerfile -t mongo-micro:v1 . 
docker run -d --rm -p 8082:8082 mongo-micro:v1

docker build -f microservices/clickhouse/Dockerfile -t click-micro:v1 . 
docker run -d --rm -p 8081:8081 click-micro:v1

docker build -f main/Dockerfile -t chatgo:v1 . 
docker run --rm -it -p 8080:8080 chatgo:v1