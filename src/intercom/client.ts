const io = require('socket.io-client');
class SocketClient {

    public static socket;

    public setup() {
        SocketClient.socket = io.connect('https://socket.purecore.io/', { path: "/hosting" });

        /**
         * Authentication process:
         * 1. connect
         * 2. authenticate with hash
         * 3. set as host machine when authenticated
         */
        SocketClient.socket.on('connect', function () {
            Supervisor.emitter.emit('socketConnected')
            Supervisor.emitter.emit('socketAuthenticating')
            SocketClient.socket.emit('auth', Supervisor.hash)
        });
        SocketClient.socket.on('authenticated', function (data) {
            Supervisor.emitter.emit('socketAuthenticated')
            Supervisor.emitter.emit('socketHostRequest')
            SocketClient.socket.emit('host')
        });
        SocketClient.socket.on('hosting', function (data) {
            Supervisor.emitter.emit('socketHosting')
        });

        /**
         * Admin Events
         */
        SocketClient.socket.on('createContainer', function (data) {
            console.log(data);
        });

        /**
         * User Events
         */
        SocketClient.socket.on('restartContainer', function (data) {
            console.log(data);
        });
        SocketClient.socket.on('startContainer', function (data) {
            console.log(data);
        });
        SocketClient.socket.on('stopContainer', function (data) {
            console.log(data);
        });
        SocketClient.socket.on('pauseContainer', function (data) {
            console.log(data);
        });
        SocketClient.socket.on('resumeContainer', function (data) {
            console.log(data);
        });

        /** 
         *  Status Updates
         */
        SocketClient.socket.on('disconnect', function () { Supervisor.emitter.emit('socketDisconnected') });
    }

}