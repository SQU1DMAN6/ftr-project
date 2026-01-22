# Improved FtRDeploy with SSH private key support when using sudo

import os
import sys
from ftplib import FTP
import paramiko
import getpass

# Globals
target_ip = None
target_port = 22
file_path = None
username = None
password = None
private_key_path = None
use_key = False
deploy_path = None


def load_private_key(path):
    try:
        return paramiko.RSAKey.from_private_key_file(path)
    except Exception:
        try:
            return paramiko.Ed25519Key.from_private_key_file(path)
        except Exception as e:
            print(f"[!] Failed to load private key: {e}")
            return None


def ssh_connect():
    ssh = paramiko.SSHClient()
    ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())

    if use_key and private_key_path:
        key = load_private_key(private_key_path)
        if key is None:
            return None
        ssh.connect(target_ip, port=target_port, username=username, pkey=key)
    else:
        ssh.connect(target_ip, port=target_port, username=username, password=password)

    return ssh


def deploy_file_ssh():
    print(f"[*] Connecting to {target_ip}:{target_port} via SSH...")

    ssh = ssh_connect()
    if ssh is None:
        print("[!] SSH connection failed.")
        return

    ftp = ssh.open_sftp()
    filename = os.path.basename(file_path)
    remote_file = os.path.join(deploy_path, filename)
    ftp.put(file_path, remote_file)
    ftp.close()
    ssh.close()

    print(f"[+] File uploaded to {remote_file} via SSH.")


def deploy_file_ftp():
    print(f"[*] Connecting to FTP at {target_ip}:{target_port}...")

    ftp = FTP()
    ftp.connect(target_ip, target_port, timeout=10)
    ftp.login(username, password)

    try:
        ftp.cwd(deploy_path)
    except Exception as e:
        print(f"[!] Failed to change to {deploy_path}: {e}")
        ftp.quit()
        return

    with open(file_path, "rb") as f:
        filename = os.path.basename(file_path)
        print(f"[*] Uploading {filename}...")
        ftp.storbinary(f"STOR {filename}", f)

    ftp.quit()
    print(f"[+] File uploaded to {deploy_path} via FTP.")


def start_remote():
    if target_port == 22 or target_port == 2222:
        filename = os.path.basename(file_path)
        remote_file = os.path.join(deploy_path, filename)

        ssh = ssh_connect()
        if ssh is None:
            print("[!] SSH connection failed.")
            return

        print(f"[*] Running {remote_file} on target...")

        stdin, stdout, stderr = ssh.exec_command(f"python3 {remote_file}")
        print(stdout.read().decode())
        print(stderr.read().decode())
        ssh.close()
    else:
        print("[!] Remote execution only supported over SSH (port 22 or 2222).")


print("Welcome to FtR v3 (Improved)\nWritten by Quan Thai | https://quanthai.net/\n")

while True:
    try:
        command = input("ftr> ").strip()

        if command.startswith("port "):
            target_port = int(command.split(" ")[1])
            print(f"[+] Port set to {target_port}")

        elif command.startswith("contact "):
            target_ip = command.split(" ")[1]
            print(f"[+] Target set to {target_ip}")
            if not target_ip.replace(".", "").isdigit():
                print("[*] Domain-style target detected.")

            username = input("Username: ")

            key_choice = input("Use private key? (y/N): ").lower()
            if key_choice == "y" or key_choice == "Y":
                use_key = True
                private_key_path = input("Private key path: ").strip()
                password = None
            else:
                use_key = False
                password = getpass.getpass("Password: ")

            deploy_path = input("Remote deploy path (e.g. /home/user/Desktop/): ").strip()
            print(f"[+] Credentials + path stored. Ready to deploy.")

        elif command.startswith("file "):
            file_path = command.split(" ", 1)[1]
            if os.path.exists(file_path):
                print(f"[+] File set to {file_path}")
            else:
                print("[!] File not found.")

        elif command == "deploy":
            if not target_ip or not file_path:
                print("[!] Missing target or file.")
            elif target_port == 21:
                deploy_file_ftp()
            else:
                deploy_file_ssh()

        elif command == "start":
            if not file_path:
                print("[!] No file to execute.")
            else:
                start_remote()

        elif command == "99":
            print("Exiting...")
            sys.exit(0)

        else:
            print("[?] Unknown command.")

    except KeyboardInterrupt:
        print("\n[!] Interrupted.")
        break
