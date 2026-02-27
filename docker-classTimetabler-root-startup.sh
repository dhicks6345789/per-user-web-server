# This script runs as root when the image starts up.

if [ ! -d "/root/class-timetabler" ]; then
  cd /root
  git clone https://github.com/ukfootprint/class-timetabler.git
  #rm -rf /root/class-timetabler/backend/venv
  
  "dev": "vite --host"
  /root/class-timetabler/frontend/package.json
fi

if [ ! -d "/root/class-timetabler/backend/venv" ]; then
  # Install the Class Timetabler backend.
  cd /root/class-timetabler/backend
  python3 -m venv venv
fi

cd /root/class-timetabler/backend
source venv/bin/activate
pip install -r requirements.txt

# Install the Class Timetabler frontend.
cd /root/class-timetabler/frontend
npm install

cd /root/class-timetabler
./dev.sh
