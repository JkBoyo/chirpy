# Chripy Server
This is my implementation of the [Boot.dev](https://boot.dev) [Learn HTTP Servers in Go](https://www.boot.dev/courses/learn-http-servers-golang).

The course has you write this Go server as a teaching tool for showing the concepts of how backend servers work.

# Sever Install
To install the server there are a few things that are needed.

1. The [Go](https://go.dev) toolchain
2. A [Postgres](https://postgresql.org) db
3. [Goose](https://github.com/pressly/goose) migration tool
4. git

With these tools installed you should be able to

1. Clone down the repo to your local dir `git clone https://github.com/jkboyo/chirpy /path/to/dest/dir`
2. Run migrations from inside the `./chirpy/sql/schema` dir using the `goose postgres postgres://username:password@databaseaddr:5432/chirpy up`.
3. From there you should set up a `.env` file with the format of 
```go
DB_URL="postgres://username:password@databaseaddr:5432/chirpy?sslmode=disable"
PLATFORM="user"
SECRET="Randomsecretstringhere"
POLKA_KEY="GivenPolkaKey"
```
This will allow the db to connect and prevent you from being able to use the `/admin/reset` endpoint. If you wish to be able to use this endpoint change `PLATFORM` to equal "dev".

At this point you should be able to run the server and see how it works!

The api's endpoints are documented below for examples on how to make it work.

# Server Paths
## /admin/metrics
### GET
This endpoint returns the number of unique hits that have been made to the `/app` path.

## /admin/reset
### POST
This endpoint attempts to reset the db but fails with 403 if the `.env` file doesn't contain the variable "PLATFORM"="dev".

## /api/healthz
### POST
Responds with 200 if the server is up and running.

## /api/users
### POST
Takes a request with the form 
```json
{
"password": "strong123",
"email": "coolmail@gmail.com"
}
```
to create a user.
### PUT
Takes a request with the same form as above and updates the user with the same email.

## /api/login
### POST
Takes the same json as the `POST /api/users` endpoint above and returns a token for authorization.

## /api/chirps
### POST
Takes a json request with the below form
```json
{
"body": "What an awesome chirp btw"
}
```
This `body` can't be longer than 140 chars and if any of the words say "Kerfuffle", "Sharbert", or "Fornax" they will be changed to "****".
The request will return json with the below structure.
```json
{
"id": "e3a91e99-6733-43d3-9286-fbe8efa7400d",
"created_at": "2012-10-31 15:50:13.793654 +0000 UTC",
"updated_at": "2012-10-31 15:50:13.793654 +0000 UTC",
"body": "What an awesome chirp btw",
"user_id": "b3a99492-738b-4c2a-b7ee-8532854c919c",
}
```

### GET
This endpoint fetches either all chirps and returns them in a json array with the json structure
```json
{
"id": "e3a91e99-6733-43d3-9286-fbe8efa7400d",
"created_at": "2012-10-31 15:50:13.793654 +0000 UTC",
"updated_at": "2012-10-31 15:50:13.793654 +0000 UTC",
"body": "What an awesome chirp btw",
"user_id": "b3a99492-738b-4c2a-b7ee-8532854c919c",
}
```
for each element, or you can add a query `?author_id=someuuid` that filters based on the passed in uuid.

## /api/chirps/{chirp_id}
### GET
This endpoint allows you to return a chirp based on the id passed in the path and returns the basic chirp json.
```json
{
"id": "e3a91e99-6733-43d3-9286-fbe8efa7400d",
"created_at": "2012-10-31 15:50:13.793654 +0000 UTC",
"updated_at": "2012-10-31 15:50:13.793654 +0000 UTC",
"body": "What an awesome chirp btw",
"user_id": "b3a99492-738b-4c2a-b7ee-8532854c919c",
}
```
### DELETE
Allows only the author to delete the chirp with the specified id.

## /api/refresh
### POST
Refreshes the user access token.

## /api/revoke
### POST
Revokes the user refresh token.

## /api/polka/webhooks
### POST
Listens for payment information from the "polka payment service" which is a made up example to showcase how to use webhooks.
This enpoint checks against your `POLKA_KEY` in your `.env` to make sure that only requests with it in the authorization header function.
To simulate requests from Polka you will need to make requests yourself with your `POLKAKEY` as the bearer token with the structure of
```json
{
  "data": {
    "user_id": "username"
  },
  "event": "user.upgraded"
}
```
