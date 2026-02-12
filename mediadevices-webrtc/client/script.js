const ws = new WebSocket("ws://192.168.2.92:9091/ws");

let peerConnection = null;
let pendingRemoteIceCandidates = [];

ws.onopen = () => console.log("websocket connected");
ws.onclose = () => console.log("websocketdisconnected");
ws.onerror = (event) => console.log("ws error:", event);

const videoElement = document.getElementById("videoElement");
const videoSection = document.getElementById("videoSection");
const audioElement = document.getElementById("audioElement");

ws.onmessage = async (event) => {
  console.log(event);

  const message = JSON.parse(event.data);
  console.log("message recieved on socket-" + message.type);

  switch (message.type) {
    case "offer":
      console.log("processing offer");
      try {
        peerConnection = new RTCPeerConnection({
          iceServers: [],
        });
        console.log("peer connection created");

        peerConnection.onicecandidate = async (e) => {
          if (e.candidate == null) return;
          console.log("------------ice generated at client------------");
          console.log(JSON.stringify(e.candidate));
          console.log("------------ice generated at client------------");
          console.log("onicecandidate:" + e);

          const iceMessage = {
            type: "ice",
            data: e.candidate,
          };
          ws.send(JSON.stringify(iceMessage));
          console.log("ice message send from client");
        };

        peerConnection.onconnectionstatechange = () => {
          console.log(
            "connection state change:",
            peerConnection.connectionState,
          );
        };

        peerConnection.ontrack = (e) => {
          console.log("Track received:", e.track.kind);
          console.log(e);

          if (e.track.kind === "audio") {
            console.log("+++++++++++++audio transceiver++++++++++++++");
            const stream = new MediaStream();
            stream.addTrack(e.track);
            console.log("`````````````````````````");
            console.log(stream);
            console.log(e.streams[0]);
            console.log("`````````````````````````");

            audioElement.srcObject = stream;
            audioElement.muted = false;
            audioElement.play().catch((err) => {
              console.log("remote audio playing err");
              console.log(err);
            });
            console.log("audio stream set");
          }

          if (e.track.kind === "video") {
            console.log("+++++++++++++camera transceiver++++++++++++++");
            const stream = new MediaStream();
            stream.addTrack(e.track);
            console.log("`````````````````````````");
            console.log(stream);
            console.log(e.streams[0]);
            console.log("`````````````````````````");
            videoElement.srcObject = stream;
            videoElement.play().catch((err) => {
              console.log("remote video playing err");
              console.log(err);
            });
            console.log("camvideo stream set");
          }
        };

        peerConnection.addTransceiver("audio", {
          direction: "recvonly",
        });

        peerConnection.addTransceiver("video", {
          direction: "recvonly",
        });
        await peerConnection.setRemoteDescription(message.data);
        console.log("remote description set on offer message");
        const answer = await peerConnection.createAnswer();
        await peerConnection.setLocalDescription(answer);
        const answerMessage = {
          type: "answer",
          data: answer,
        };
        ws.send(JSON.stringify(answerMessage));
      } catch (error) {
        console.error("Failed to set remote description:", error);
      }

      for (const remoteCandidate of pendingRemoteIceCandidates) {
        console.log("*********************");
        console.log(remoteCandidate);
        console.log("*********************");

        try {
          await peerConnection.addIceCandidate(
            new RTCIceCandidate(remoteCandidate),
          );
          console.log("remote ICE candidate added on answer message");
        } catch (iceErr) {
          console.warn("Failed to add ICE candidate:", iceErr);
        }
      }
      pendingRemoteIceCandidates = [];
      break;

    case "ice":
      console.log("processing ice");
      console.log(message);
      try {
        if (peerConnection && peerConnection.remoteDescription) {
          await peerConnection.addIceCandidate(
            new RTCIceCandidate(message.data),
          );
          console.log("remote ICE candidate added immediately");
        } else {
          pendingRemoteIceCandidates.push(message.data);
          console.log("remote ICE candidate buffered");
        }
      } catch (iceErr) {
        console.log("Failed to add ICE candidate:", iceErr);
      }
      break;
    default:
      console.log("undefined case");
      break;
  }
};
