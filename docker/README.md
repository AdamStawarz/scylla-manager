# Scylla Manager

[Scylla Manager](https://docs.scylladb.com/operating-scylla/manager/) is a product for database operations automation tool for [ScyllaDB](https://www.scylladb.com/).
It can schedule tasks such as repairs and backups. Scylla Manager can manage multiple Scylla clusters and run cluster-wide tasks in a controlled and predictable way.

Scylla Manager is available for Scylla Enterprise customers and Scylla Open Source users.
With Scylla Open Source, Scylla Manager is limited to 5 nodes.
See the [Scylla Manager Proprietary Software](https://www.scylladb.com/scylla-manager-software-license-agreement) for details.

## Docker Compose Example
[Docker compose](https://docs.docker.com/compose/) is a tool for defining and running multi-container applications without having to orchestrate the participating containers by hand.

__Purpose of the example__

This example uses single node Scylla cluster and MinIO and should not be used in a production setting.
Please see the [Scylla Manager Operations Guide](https://docs.scylladb.com/operating-scylla/manager/) for a proper production setup.
Once you have the example up and running you can try out the various commands for running repairs and backups that Scylla Manager provides.

__Procedure__

1. Copy the following yaml and save it to your current working directory as `docker-compose.yaml`.
```yaml
version: "3.7"

services:
  scylla-manager:
    image: scylladb/scylla-manager:${SCYLLA_MANAGER_VERSION}
    networks:
      public:
    depends_on:
      - scylla-manager-db

  scylla-manager-db:
    image: scylladb/scylla:latest
    volumes:
      - scylla_manager_db_data:/var/lib/scylla
    networks:
      public:
    command: --smp 1 --memory 1G

  scylla:
    build:
      context: .
    image: scylladb/scylla-with-agent
    volumes:
      - scylla_data:/var/lib/scylla
    networks:
      public:
        ipv4_address: 192.168.100.100
    command: --smp 1

  minio:
    image: minio/minio:latest
    volumes:
      - minio_data:/data
    networks:
      public:
    ports:
      - "9001:9000"
    environment:
      MINIO_ACCESS_KEY: minio
      MINIO_SECRET_KEY: minio123
    command: server /data

volumes:
  minio_data:
  scylla_data:
  scylla_manager_db_data:

networks:
  public:
    driver: bridge
    ipam:
      driver: default
      config:
        - subnet: 192.168.100.0/24
```

This instructs Docker to create two containers, a scylla-manager container that runs the Scylla Manager Server and scylla-manager-db container that runs the Scylla database that Scylla Manager uses to save its data.
It also gives you a [MinIO](https://min.io/) instance for backups and a scylla instance to use for your application.
Please bear in mind that this is not a production setup as that would most require much more and advanced configuration of storage and security.

2. Copy the following `Dockerfile` to the same directory as the `docker-compose.yaml` file you saved in item 1.
This docker file is used to create a ScyllaDB image with the Scylla Manager Agent patched on top of it.
The reason for this is that the Agent needs access to ScyllaDB's data files in order to create backups.
```Dockerfile
FROM scylladb/scylla:latest

RUN echo -e "#!/usr/bin/env bash\n\
set -eu -o pipefail\n\
if [[ ! -f \"/var/lib/scylla-manager/scylla_manager.crt\" || ! -f \"/var/lib/scylla-manager/scylla_manager.key\" ]]; then\n\
   /sbin/scyllamgr_ssl_cert_gen\n\
fi\n\
exec /usr/bin/scylla-manager-agent $@" > /scylla-manager-agent-docker-entrypoint.sh

RUN echo -e "[scylla-manager]\n\
name=Scylla Manager for Centos \$releasever - \$basearch\n\
baseurl=http://downloads.scylladb.com/manager/rpm/unstable/centos/branch-2.0/latest/scylla-manager/\7/\$basearch/\n\
enabled=1\n\
gpgcheck=0\n" > /etc/yum.repos.d/scylla-manager.repo

RUN echo -e "[program:scylla-manager-agent]\n\
command=/scylla-manager-agent-docker-entrypoint.sh\n\
autorestart=true\n\
stdout_logfile=/dev/stdout\n\
stdout_logfile_maxbytes=0\n\
stderr_logfile=/dev/stderr\n\
stderr_logfile_maxbytes=0" > /etc/supervisord.conf.d/scylla-manager-agent.conf

RUN yum -y install epel-release && \
    yum -y clean expire-cache && \
    yum -y update && \
    yum install -y scylla-manager-agent && \
    yum clean all && \
    rm /etc/yum.repos.d/scylla-manager.repo

RUN echo -e "auth_token: token\n\
s3:\n\
    access_key_id: minio\n\
    secret_access_key: minio123\n\
    endpoint: http://minio:9000" > /etc/scylla-manager-agent/scylla-manager-agent.yaml

RUN rm -f /var/lib/scylla-manager/scylla_manager.crt && \
    rm -f /var/lib/scylla-manager/scylla_manager.key && \
    chmod --reference=/usr/bin/scylla-manager-agent /scylla-manager-agent-docker-entrypoint.sh
```

3. Create and start the containers by running the `docker-compose up` command.
```bash
SCYLLA_MANAGER_VERSION=2.0.1 docker-compose up -d
```

4. Verify that Scylla Manager started by using the `logs` command.
```bash
docker-compose logs -f scylla-manager
```

## Docker example

It is quite possible to setup this using docker directly without letting Docker Compose do the heavy lifting.

To avoid bootstrapping issues it can be best to start with periferal services first. In this case MinIO.
Start MinIO in a container like this and link it to the scylla instance you created above.
```bash
docker run -d -p 9000:9000 --name minio1 \
    -e "MINIO_ACCESS_KEY=minio" \
    -e "MINIO_SECRET_KEY=minio123" \
    -v /mnt/data:/data minio/minio server /data
```

Now you need to copy the same `Dockerfile` that the Docker Compose example uses and save it to your working directory.
This `Dockerfile` patches a regular ScyllaDB image with a Scylla Manager Agent to allow for proper communication between ScyllaDB and Scylla Manager.

Execute the build like this:
```bash
docker build -t scylladb/scylla-with-agent .
```

Create a new ScyllaDB instance using the image you just built. This instance will hold the data of your own application.
We need to link it with the MinIO instance to allow the Scylla Manager Agent to access the MinIO instance.
```bash
docker run -d --name scylla --link minio1 --mount type=volume,source=scylla_db_data,target=/var/lib/scylla scylladb/scylla-with-agent --smp 1 --memory=1G
```

Now you can start a regular ScyllaDB instance that Scylla Manager will use to store it's internal data in with the following command.
```bash
docker run -d --name scylla-manager-db --mount type=volume,source=scylla_manager_db_data,target=/var/lib/scylla scylladb/scylla --smp 1 --memory=1G
```

Finally it's time to start Scylla manager using the following command. We need to link this instance to both of the ScyllaDB instances.
```bash
docker run -d --name scylla-manager --link scylla-manager-db --link scylla scylladb/scylla-manager:2.0.1
```

## Using sctool

Use docker exec to invoke bash in the `scylla-manager` container to add the one node cluster you created above to Scylla Manager.
```bash
docker exec -it scylla-manager sctool cluster add -c cluster --host=scylla --auth-token=token
1a0feeba-5b38-4cc4-949e-6bd704667552
 __  
/  \     Cluster added! You can set it as default, by exporting its name or ID as env variable:
@  @     $ export SCYLLA_MANAGER_CLUSTER=1a0feeba-5b38-4cc4-949e-6bd704667552
|  |     $ export SCYLLA_MANAGER_CLUSTER=<name>
|| |/    
|| ||    Now run:
|\_/|    $ sctool status -c cluster
\___/    $ sctool task list -c cluster
docker exec -it scylla-manager  sctool status -c cluster
Datacenter: datacenter1
+----------+-----+----------+-----------------+
| CQL      | SSL | REST     | Host            |
+----------+-----+----------+-----------------+
| UP (0ms) | OFF | UP (0ms) | 192.168.100.100 |
+----------+-----+----------+-----------------+
```

See the complete [sctool reference](https://docs.scylladb.com/operating-scylla/manager/2.0/sctool/) for further details.