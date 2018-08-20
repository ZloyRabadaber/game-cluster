docker exec proxy certbot-certonly --preferred-challenges=http-01 --expand --domain naogames.ru  --email afternao@gmail.com
docker exec proxy haproxy-refresh
