import os
import sys
import json
import pooch
from pathlib import Path
from loguru import logger
from pydactyl import PterodactylClient

# Pterodactyl API
API_URL = os.environ.get("PTERODACTYL_API_URL")
API_TOKEN = os.environ.get("PTERODACTYL_API_TOKEN")

# Server Configs
SERVER_ID = os.environ.get("PTERODACTYL_ID")

# Path Configs
CONFIG_PATH = Path(os.environ.get("MAP_CONFIG_PATH"))
CONFIG_PATH_ARCHIVE = CONFIG_PATH.joinpath("server-backup.tar.gz")

api = PterodactylClient(API_URL, API_TOKEN)

def pydactyl_get_latest_backup_attributes() -> str:
    raw_data = api.client.servers.backups.list_backups(SERVER_ID)
    logger.debug(raw_data)
    latest_backup_attributes = raw_data["data"][0]["attributes"]
    logger.debug(latest_backup_attributes)
    if latest_backup_attributes["is_successful"]:
        return latest_backup_attributes
    else:
        logger.error("Latest backup is not successful!")
        sys.exit(1)

def pydactyl_download_backup(backup_data: dict):
    response_json = api.client.servers.backups.get_backup_download(SERVER_ID, backup_data["uuid"])
    checksum = backup_data["checksum"]
    signed_url = response_json["attributes"]["url"]

    pooch.retrieve(signed_url, checksum, CONFIG_PATH_ARCHIVE.name, CONFIG_PATH, progressbar=True)

def main():
    backup_detail = pydactyl_get_latest_backup_attributes()
    logger.debug(json.dumps(backup_detail, indent=2))
    pydactyl_download_backup(backup_detail)

main()
