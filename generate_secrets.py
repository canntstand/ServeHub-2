import string
import secrets

TARGET_FILE = "./ansible/vars/secrets.yml"
CHARS = string.ascii_letters + string.digits + "!@#$%^&*()-_=+"

def generate_secure_password(length=24):
    return "".join(secrets.choice(CHARS) for _ in range(length))

vars_to_generate = {
    "admin_password",
    "matrix_bot_password",
    "monitoring_secret",
    "postgres_password",
    "synapse_registration_shared_secret",
    "synapse_macaroon_secret_key",
    "synapse_form_secret",
    "secret_vaultwarden_password",
}

generated_secrets = {var_name: generate_secure_password() for var_name in vars_to_generate}

yml_to_generate = '''
vps_public_ip: ""
vps_user: ""
vps_root_password: ""

local_private_ip: ""
local_user: ""
local_root_password: ""

synapse_server_name: ""
admin_user: ""
admin_password: "{admin_password}"

email: ""

smtp_host: "smtp.gmail.com"
smtp_port: "465"
smtp_user: ""
smtp_password: ""
smtp_from: ""
smtp_to: ""

matrix_bot_username: "alertbot"
matrix_bot_password: "{matrix_bot_password}"
monitoring_secret: "{monitoring_secret}"

webnames_apikey: ""

postgres_db_nextcloud: "nextcloud"
postgres_db_synapse: "synapse"
postgres_db_vaultwarden: "vaultwarden"
postgres_user: "r9888"
postgres_password: "{postgres_password}"

synapse_registration_shared_secret: "{synapse_registration_shared_secret}"
synapse_macaroon_secret_key: "{synapse_macaroon_secret_key}"
synapse_form_secret: "{synapse_form_secret}"

secret_vaultwarden_password: "{secret_vaultwarden_password}"

ssh_public_key: ""
'''

rendered_yml = yml_to_generate.format(**generated_secrets)

with open(TARGET_FILE, "w", encoding="utf-8") as file:
    file.write(rendered_yml)