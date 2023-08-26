echo "PasswordAuthentication no" >> /etc/ssh/sshd_config

addgroup -g 1000 linsk
adduser -G linsk linsk -S -u 1000