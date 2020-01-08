# hostmonitor
Host monitoring tool using http methods

# Usage

```
hostmonitor -port [portnumber]
```

The tool will expose an API listener on localhost and your chosen port to which commands can be submitted.

Add a monitor by issuing a request:
```
curl --header "Content-Type: application/json" --request POST --data '{"id": 1,"url": "https://httpbin.org/status/200","frequency": 60,"timeout": 3}' http://127.0.0.1:8085/addmonitor/http/GET
```

Check response
```
{"Ok":true,"Message":"Monitor added"}
```

Query the list of watched monitors:

```
curl http://127.0.0.1:8085/monitors
```

Parse the JSON response:

```
{
"monitors": {
    "3": {
        "id": 3,
        "alive": true,
        "message": "",
        "change": "",
        "latency": 119,
        "lastChange": "2020-01-08T15:22:15.2914339+02:00",
        "timestamp": "2020-01-08T15:24:15.4133467+02:00",
        "code": 200,
        "status": "200 OK"
    }
  }
}
```

# Todo

1.Add data persistence

2.Add testing via POST with payload

3.Add keyword checking of the test response
