## Generate a JWT keypair
```bash
mkdir -p .secrets
openssl genrsa 4096 > .secrets/priv.pem
openssl rsa -in .secrets/priv.pem -pubout > .secrets/pub.pem
```

## Config file
```json
{
    "server_config": {
        "host": "127.0.0.1",
        "port": 8080
    },
    "jwt_private_key_path": ".secrets/priv.pem",
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

```bash
docker run -p 8080:8080 -v .secrets:/secrets --name sms-issuer sms-issuer
```
