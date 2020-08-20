const http = require('http');
const https = require('https');
const socketio = require('socket.io');
const app = require('express')();

class SocketServer {

    public static io;
    public static authenticated = [];

    public getSocket(server) {
        return new socketio(server).on('connection', client => {
            console.log(client)
            client.on('authenticate', authInfo => {
                this.authenticate(client, authInfo);
            });
            client.on('host', hostRequest => {
                if (SocketServer.isAuthenticated(client)) DockerHelper.createContainer(hostRequest).catch((err) => { /*ignore*/ })
            });
            client.on('disconnect', () => { SocketServer.removeAuth(client.id) });
        })
    }

    public static isAuthenticated(client) {
        return SocketServer.authenticated.includes(client.id);
    }

    public static addAuth(clientid) {
        if (!SocketServer.authenticated.includes(clientid)) {
            Supervisor.emitter.emit('clientConnected');
            this.authenticated.push(clientid);
        }
    }

    public static removeAuth(clientid) {
        if (!SocketServer.authenticated.includes(clientid)) {
            Supervisor.emitter.emit('clientDisconnected');
            SocketServer.authenticated = SocketServer.authenticated.filter(x => x !== clientid);
        }
    }

    public authenticate(client, authInfo) {
        if ((authInfo == null || authInfo == "" || authInfo == [] || authInfo == {})) {
            const accepetedHostnames = ["api.purecore.io", "purecore.io"]
            const hostname = client.handshake.headers.host.split(".").shift();
            if (accepetedHostnames.includes(hostname)) {
                SocketServer.addAuth(client.id)
            } else {
                client.disconnect()
            }
        }
    }

    public setup() {
        try {
            Supervisor.emitter.emit('creatingServer');
            const server = this.getHTTP();
            Supervisor.emitter.emit('createdServer');

            try {
                Supervisor.emitter.emit('creatingSocketServer');
                SocketServer.io = this.getSocket(server);
                server.listen(31518, () => {
                    Supervisor.emitter.emit('createdSocketServer');
                }).on('error', function (error) {
                    Supervisor.emitter.emit('errorCreatingSocketServer', new Error(error.code));
                });
            } catch (error) {
                Supervisor.emitter.emit('errorCreatingSocketServer', error);
            }
        } catch (error) {
            Supervisor.emitter.emit('errorCreatingServer', error);
        }
    }

    public getHTTP(): any {
        let httpServer = null;
        const cert = this.getCert();

        if (cert != null) {
            Supervisor.emitter.emit('certUse');
            httpServer = https.Server({
                key: cert.key,
                cert: cert.cert,
                ca: cert.ca
            }, app);
        } else {
            Supervisor.emitter.emit('certUnknown');
        }

        if (httpServer == null) {
            httpServer = http.Server(app);
        }
        return httpServer;
    }

    public getCert(): Cert {
        Supervisor.emitter.emit('certLoading');
        const fs = require("fs");
        const letsencryptBase = "/etc/letsencrypt/"
        try {
            if (fs.existsSync(letsencryptBase) && fs.existsSync(letsencryptBase + "live/")) {
                let files = fs.readdirSync(letsencryptBase + "live/");
                let folders = [];
                files.forEach(file => {
                    if (fs.lstatSync(letsencryptBase + file).isDirectory()) folders.push(file);
                });
                if (folders != null && Array.isArray(folders) && folders.length > 0) {
                    Supervisor.emitter.emit('certFound');
                    return new Cert(fs.readFileSync(letsencryptBase + "live/" + folders[0] + "/privkey.pem"), fs.readFileSync(letsencryptBase + "live/" + folders[0] + "/cert.pem"), fs.readFileSync(letsencryptBase + "live/" + folders[0] + "/chain.pem"));
                } else {
                    Supervisor.emitter.emit('certNotSetup');
                    return null;
                }
            } else {
                Supervisor.emitter.emit('certNotInstalled');
                return null;
            }
        } catch (error) {
            Supervisor.emitter.emit('certReadingError');
            return null;
        }
    }

}