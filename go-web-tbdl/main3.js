
var ws = new WebSocket("ws://localhost:6080/ws");
var peerConnection = new RTCPeerConnection({
    iceServers: [
        {
            urls: "stun:freeturn.net:3478"
        },
        {
            urls: "turn:freeturn.net:3478",
            username: "free",
            credential: "free"
        },
    ]
});
// Temporarily make it accessible globally for debugging:
window.myPeerConnection = peerConnection

ws.onmessage = function (event) {
    // console.log("Received: " + event.data);
    // Parse the incoming message from JSON string to JavaScript object
    var message = JSON.parse(event.data);

    // Access the type and data of the message
    console.log("Message Type: " + message.type);
    console.log("Message Data: " + message.data);

    // Handle the message based on its type
    switch (message.type) {
        case "offer": {
            // console.log("Offer received:", message.data);

            // Assuming we have a peerConnection instance of RTCPeerConnection already created
            var offerDesc = new RTCSessionDescription(JSON.parse(message.data));
            // console.log(offerDesc, 1111)

            peerConnection.setRemoteDescription(offerDesc).then(function () {
                // Once the remote description is set, create an answer
                console.log(peerConnection.remoteDescription, 11112222)

                peerConnection.ontrack = function (event) {
                    console.log("Received remote track:", event.streams[0]);
                    var remoteStream = new MediaStream();
                    remoteStream.addTrack(event.track[0]);
                    var audioElement = document.createElement("audio");
                    audioElement.srcObject = remoteStream;
                    audioElement.play();
                };
                window.myPeerConnection.getTransceivers().forEach(t => {
                    console.log(`Transceiver mid=${t.mid}, kind=${t.receiver.track.kind}, trackId=${t.receiver.track.id}, trackEnabled=${t.receiver.track.enabled}`);
                    // Check if the track is audio or video and select the appropriate element
                    let mediaElement = null;
                    if (t.receiver.track.kind === 'audio') {
                        mediaElement = document.getElementById('audioPlayer'); // Assuming an <audio> element with this ID exists
                    } else if (t.receiver.track.kind === 'video') {
                        mediaElement = document.getElementById('videoPlayer'); // Assuming a <video> element with this ID exists
                    }

                    if (mediaElement) {
                        // Attach the track to the media element
                        const stream = new MediaStream([t.receiver.track]);
                        mediaElement.srcObject = stream;
                    }
                });
                window.myPeerConnection.getReceivers().forEach(r => {
                    console.log(`Receiver Track kind=${r.track.kind}, id=${r.track.id}, enabled=${r.track.enabled}`);
                });


                return peerConnection.createAnswer();
            })
                .then(function (answer) {
                    // Set the local description with the created answer
                    return peerConnection.setLocalDescription(answer);
                })
                .then(function () {
                    // Send the answer back to the server
                    console.log(peerConnection.localDescription, 22112222)
                    var answerMessage = {
                        type: "answer",
                        data: JSON.stringify(peerConnection.localDescription)
                    };
                    console.log(answerMessage)

                    ws.send(JSON.stringify(answerMessage));
                })
                .then(() => {
                    ws.send(JSON.stringify({
                        type: "reqice",
                        data: "Start sending ice candidates"
                    }))
                })
                .catch(function (err) {
                    console.error("Error handling the offer: ", err);
                });

            break;
        }
        case "candidate": {
            console.log("ICE candidate received:", message.data);

            function addCandidate() {
                var candidate = new RTCIceCandidate(JSON.parse(message.data));
                peerConnection.addIceCandidate(candidate).catch(function (err) {
                    console.error("Error adding received ICE candidate:", err);
                });
            }

            // Check if both local and remote descriptions are set
            if (peerConnection.signalingState === "stable" || peerConnection.signalingState === "have-local-offer") {
                addCandidate();
            } else {
                console.log("Waiting for local and remote descriptions to be set before adding candidates");

                // Option 1: Queue the candidate and add it later
                // This would require implementing a queue to hold candidates until they can be added.

                // Option 2: Use an event listener to wait for the descriptions to be set
                // This example will add the candidate once the signaling state changes to stable.
                // Note: You need to remove this listener when appropriate to avoid leaks.
                var signalingStateListener = function () {
                    if (peerConnection.signalingState === "stable") {
                        addCandidate();
                        peerConnection.removeEventListener("signalingstatechange", signalingStateListener);
                    }
                };

                peerConnection.addEventListener("signalingstatechange", signalingStateListener);
            }
            break;
        }
        case "reqice": {
            console.log("Request for ICE candidates received:", message.data);

            // Listen for local ICE candidates on the peer connection
            peerConnection.onicecandidate = function (event) {
                // if (event.candidate) {
                console.log("New ICE candidate:", event.candidate);

                // Send the ICE candidate to the remote peer
                var candidateMessage = {
                    type: "candidate",
                    data: JSON.stringify(event.candidate)
                };

                ws.send(JSON.stringify(candidateMessage));
                // }
            };
            break;
        }
    }
};
ws.onopen = function (event) {
    ws.send(JSON.stringify({
        type: "Initiation",
        data: "Initiation of WebRTC"
    }));
};