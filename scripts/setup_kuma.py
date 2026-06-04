from uptime_kuma_api import UptimeKumaApi, UptimeKumaException
import os
import time

KUMA_URL = "http://127.0.0.1:51822/uptime-kuma"
ADMIN_USER = os.getenv("ADMIN_USER")
ADMIN_PASS = os.getenv("ADMIN_PASSWORD")

def setup_kuma():
    max_retries = 12
    for attempt in range(1, max_retries + 1):
        api = None
        try:
            api = UptimeKumaApi(KUMA_URL)
            try:
                api.setup(ADMIN_USER, ADMIN_PASS)
                print("Admin user created via setup.")
            except UptimeKumaException as e:
                if "already exists" in str(e).lower() or "setup already complete" in str(e).lower():
                    print("Setup already completed, trying login.")
                else:
                    raise

            api.login(ADMIN_USER, ADMIN_PASS)
            print("Logged in successfully.")

            api.add_monitor(
                type="http",
                name="Is Local Server Alive?",
                url=f"https://{os.getenv('SYNAPSE_SERVER_NAME')}/",
                interval=60,
                conditions=[],
            )
            print("Monitor added successfully.")
            break

        except Exception as e:
            print(f"Attempt {attempt}/{max_retries} failed: {e}")
            if attempt == max_retries:
                raise
            time.sleep(5)
        finally:
            if api:
                try:
                    api.disconnect()
                except:
                    pass

if __name__ == "__main__":
    setup_kuma()