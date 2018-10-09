# motarei - Simple tcp proxy for Docker Hot deploy

Motarei tracks container's public port and proxy to there.

```
client ==>  Motarei ===================> container [new]
               |                     `=> container [old]
               | find container with label
               | and its private port
           docker api
```

Motarei always proxy to a newer container.

## How to use

run docker container with label using server-starter.

```
$ KILL_OLD_DELAY=5 start_server -- docker run -P -l app=nginx nginx
```

nginx container's private port is 80.

run Motarei.

```
$ sudo motarei --label app=nginx
```

Now you can acess to nginx via port 80.

Restart container

```
$ docker pull...
$ kill -HUP [start_server's pid]
```

Container's public port will change. Motarei will do proxy to newer public port.
