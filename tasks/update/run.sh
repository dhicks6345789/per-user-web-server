# Pandas - Python library for handling data, also Excel files.
if [ ! -e /usr/share/doc/python3-pandas ]; then
  apt-get update && apt-get install -y python3-pandas
fi
echo $PYTHONPATH
# /usr/lib/python3/dist-packages/



# The default getUsers script - generates "user1", "user2" and "user3" test data.
python3 getUsers-test.py

# Run the script that uses the user data to create users in Pangolin.
python3 setUsers.py
