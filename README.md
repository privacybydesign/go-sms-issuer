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
irma server --no-tls --no-auth=false --port=8088 --config=./local-secrets/irma-config.json


# setup sms issuer
cd backend
go run . --config ../local-secrets/local.json
```

## Configuration
```json
{
    "server_config": {
        "host": "0.0.0.0",
        "port": 8080
    },
    "jwt_private_key_path": "/secrets/private.pem",
    "issuer_id": "sms_issuer",
    "full_credential": "irma-demo.sidn-pbdf.mobilenumber",
    "attribute": "mobilenumber",
    "sms_templates": {
        "en": "Yivi verification code: %s\nOr directly via a link:\nhttps://sms-issuer.staging.yivi.app/en/#!verify:%s",
        "nl": "Yivi verificatiecode: %s\nOf direct via een link:\nhttps://sms-issuer.staging.yivi.app/nl/#!verify:%s"
    },
    "sms_backend": "dummy",
    "cm_sms_sender_config": {
        "from": "",
        "api_endpoint": "",
        "product_token": "",
        "reference": ""
    },
    "storage_type": "redis",
    "redis_config": {
        "host": "redis",
        "port": 6379,
        "password": "password"
    },
    "redis_sentinel_config": {
        "sentinel_host": "redis-sentinel",
        "sentinel_port": 26379,
        "sentinel_username": "sentinel_user",
        "password": "password123",
        "master_name": "mymaster"
    },
    "turnstile_backend": "turnstile",
    "turnstile_configuration": {
        "secret_key": "",
        "site_key": "",
        "api_url": "https://challenges.cloudflare.com/turnstile/v0/siteverify"
    }
}
```