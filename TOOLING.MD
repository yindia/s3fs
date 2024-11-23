# Tooling Used for Building This Project

To set up the development environment, please run the following command, it will install all the required tool:
```shell
make bootstrap
```

## 1. Pixi
Pixi is utilized for creating a development environment without the use of Docker. It simplifies the setup process and allows for a more straightforward development experience. You can install pixi by running below command 
```shell
curl -fsSL https://pixi.sh/install.sh | bash
```


## 2. Protobuf
Protobuf (Protocol Buffers) is heavily used to define the API specifications. Developers are required to modify the `.proto` files first to implement any changes in the API specifications for the server.

## 3. ConnectRPC
ConnectRPC is a well-known framework used for building the project. It allows for the creation of REST and gRPC servers without the overhead of complex gateways or tooling. ConnectRPC builds on familiar primitives, enabling Go handlers to integrate seamlessly into your `net/http` server while working with existing middleware, routers, and observability. TypeScript clients are designed to stay close to the Fetch API and integrate smoothly with popular UI frameworks. 

## 4. Buf CLI
Alongside ConnectRPC, the Buf CLI is employed to generate code. In this project, it is used to generate ConnectRPC Go code for both the server and client, as well as the proto stubs and definitions. You can install buf by running below command 
```shell
brew install bufbuild/buf/buf
```

## 5. Mockery
Mockery is used to generate Golang mock packages for unit tests. You can install it by running the following command:
```shell
brew install mockery
```