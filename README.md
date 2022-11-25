# Field Materials Backend Interview skeleton code

## Build & Run Server locally
```
go run .
```

## Run a sample request against the server
```
curl -X POST -H "Content-Type: application/json" -d @req.json http://localhost:8080/v1/resize
```

Now in your browser, you can check one of the returned urls!


## Coding challenge solution 

I decided to pass Async value as the argument to an endpoint without introducing new endpoints.
Then I parse the value in the handler.
Processing of the resize continues depending on the parsed value.

I have three processing methods.
- processDefaultResize is the original processing method that came with the assignment.
- processAsyncResize is my solution to the first assignment (async=true).
- processBlockResize is my solution to the second assignment (async=false).

1. return a result URL immediately (do resizing asynchronously in the background) /v1/resize?async=true

In processAsyncResize, first I save the result, and then start goroutine for each request to run in the background. I introduce an unbuffered channel to pass the value of the resized image. With unbuffered channel I prevent race condition which could occur because multiple goroutines try to write to newImage variable at the same time.


2. block and return once the underlying resize operation completes (default option) /v1/resize?async=false

In processBlockResize, I use select statement to block execution until either channel releases resized image to the variable or until timeout of 3 seconds is reached.

## How to run

To run original (default) resize:
```
curl -X POST -H "Content-Type: application/json" -d @req.json http://localhost:8080/v1/resize
```

To run async true resize:
```
curl -X POST -H "Content-Type: application/json" -d @req.json 'http://localhost:8080/v1/resize?async=true'
```

To run async false resize:
```
curl -X POST -H "Content-Type: application/json" -d @req.json 'http://localhost:8080/v1/resize?async=false'
```

Don't forget quotes so that argument can be passed on.

## Note

Sorry about the messy code. There is a lot of repetition. Did not have time to refactor.