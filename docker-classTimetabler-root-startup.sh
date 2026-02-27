# This script runs as root when the image starts up.

cd /root
git clone https://github.com/ukfootprint/class-timetabler.git
#rm -rf /root/class-timetabler/backend/venv

# Install the Class Timetabler backend.
#RUN cd /root/class-timetabler/backend && python3 -m venv venv && source venv/bin/activate && pip install -r requirements.txt

# Install the Class Timetabler frontend.
#RUN cd /root/class-timetabler/frontend && npm install
