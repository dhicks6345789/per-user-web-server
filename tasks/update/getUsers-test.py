import os
import csv

print("Generating test data.", flush=True)

if not os.path.exists("data"):
    os.makedirs("data")

with open("data/users.csv", "w", newline="\n") as usersFile:
    usersWriter = csv.writer(usersFile, delimiter=",", quotechar="\"", quoting=csv.QUOTE_ALL)
    usersWriter.writerow(["Username", "Full Name", "Default Password"])
    usersWriter.writerow(["user1", "User One", "user1"])
    usersWriter.writerow(["user2", "User Two", "user2"])
    usersWriter.writerow(["user3", "User Three", "user3"])
