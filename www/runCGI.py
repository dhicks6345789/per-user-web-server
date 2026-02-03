#!/usr/bin/python3

import subprocess
import os
import sys
import pwd

def run_as_user(uid, script_path):
    """
    Executes a script at a given path as a specific UID.
    """
    # Check if the script exists before trying to run it
    if not os.path.exists(script_path):
        print(f"Error: Script not found at {script_path}")
        return

    def set_user():
        # This function is called in the child process before the script executes
        try:
            os.setuid(uid)
        except PermissionError:
            print(f"Error: Insufficient permissions to switch to UID {uid}")
            sys.exit(1)

    try:
        # We use subprocess.run to execute the script
        # preexec_fn allows us to change the UID in the child process specifically
        result = subprocess.run(
            ['python3', script_path], 
            preexec_fn=set_user,
            capture_output=True,
            text=True
        )
        
        print("--- Output ---")
        print(result.stdout)
        
        if result.stderr:
            print("--- Errors ---")
            print(result.stderr)
            
    except Exception as e:
        print(f"An unexpected error occurred: {e}")

if __name__ == "__main__":
    print("Content-Type: text/plain\n")
    print("runCGI running.")
    print(sys.argv)

    username = sys.argv[1].split("/")[2]
    user_info = pwd.getpwnam(username)
    uid = user_info.pw_uid

    print("Username: " + username)
    
    # UID and path
    run_as_user(uid, sys.argv[1])
