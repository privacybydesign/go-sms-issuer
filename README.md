## Generate a JWT keypair
```bash
mkdir -p .secrets
openssl genrsa 4096 > .secrets/priv.pem
openssl rsa -in .secrets/priv.pem -pubout > .secrets/pub.pem
```

## Running in Docker Compose
For docker compose the secrets in `local-secrets` will be used. 
DONT USE THE KEYS FROM `local-secrets` IN PRODUCTION!

You can start the containers by running
```bash
docker compose up
```

## Running locally without Docker
You can also run this program locally without Docker. 
For that you can still use the config and secrets from the `local-secrets`.
```bash
# build and setup frontend
pushd frontend
yarn install
./build.sh
popd
cp local-secrets/sms-issuer/config.js frontend/build/assets/config.js

# setup irma server
irma server --no-tls --no-auth=false --port=8088 listen-addr=127.0.0.1 --config=./local-secrets/irma-config.json


# setup sms issuer
cd backend
go run . --config ../local-secrets/local.json
```
