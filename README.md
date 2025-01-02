## Generate a JWT keypair
```bash
mkdir -p .secrets
openssl genrsa 4096 > .secrets/priv.pem
openssl rsa -in .secrets/priv.pem -pubout > .secrets/pub.pem
```

## Running in Docker compose
Setup a local .secrets directory with the following structure:

```
.secrets
    sms-issuer
        private.pem
        config.json
    irma-server
        public.pem
        config.json
```

private.pem and public.pem form a keypair like the one generated in the step above.
The `sms-issuer/config.json` should look something like this:
```json
{
    "server_config": {
        "host": "127.0.0.1",
        "port": 8080
    },
    "jwt_private_key_path": "/secrets/priv.pem",
    "issuer_id": "sms_issuer",
    "full_credential": "irma-demo.sidn-pbdf.mobilenumber",
    "attribute": "mobilenumber",
    "sms_templates": {
        "en": "",
        "nl": ""
    },
    "cm_sms_sender_config":{
        "from":"",
        "api_endpoint":"",
        "product_token":"",
        "reference":""
    }
}
```

The `irma-server/config.json` should look something like this:
```json
{
    "requestors": {
        "sms_issuer":  {
            "auth_method": "publickey",
            "key_file": "/config/public.pem",
            "issue_perms": [
                "irma-demo.sidn-pbdf.mobilenumber"
            ]
        }
    }
}
```

