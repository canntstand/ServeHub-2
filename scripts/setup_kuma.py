from uptime_kuma_api import UptimeKumaApi
import os

KUMA_URL = "http://127.0.0.1:51822"
ADMIN_USER = os.getenv("ADMIN_USER")
ADMIN_PASS = os.getenv("ADMIN_PASSWORD")


def setup_kuma():
    api = UptimeKumaApi(KUMA_URL)

    api.login(ADMIN_USER, ADMIN_PASS)

    api.add_monitor(
        type="http",
        name="Is Local Server Alive?",
        url=f"https://{os.getenv('SYNAPSE_SERVER_NAME')}/",
        interval=60,
    )

    api.disconnect()


if __name__ == "__main__":
    setup_kuma()
