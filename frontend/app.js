const params = new URLSearchParams(location.search)
const ROOM_ID = params.get('room')

const ws = new WebSocket(`ws://${location.host}/ws/${ROOM_ID}`)

const pc = new RTCPeerConnection({
    iceServers: [{ urls: 'stun:stun.l.google.com:19302' }]
})

navigator.mediaDevices.getUserMedia({ video: true, audio: true })
    .then(stream => {
        document.getElementById('localVideo').srcObject = stream
        stream.getTracks().forEach(t => pc.addTrack(t, stream))
    })

pc.onicecandidate = ({ candidate }) => {
    if (candidate) {
        ws.send(JSON.stringify({ type: 'ice', candidate }))
    }
}

pc.ontrack = ({ streams }) => {
    document.getElementById('remoteVideo').srcObject = streams[0]
}

ws.onmessage = async ({ data }) => {
    const msg = JSON.parse(data)

    if (msg.type === 'offer') {
        await pc.setRemoteDescription(msg.sdp)
        const answer = await pc.createAnswer()
        await pc.setLocalDescription(answer)
        ws.send(JSON.stringify({ type: 'answer', sdp: answer }))

    } else if (msg.type === 'answer') {
        await pc.setRemoteDescription(msg.sdp)

    } else if (msg.type === 'ice') {
        await pc.addIceCandidate(msg.candidate)
    }
}

document.getElementById('startBtn').addEventListener('click', async () => {
    const offer = await pc.createOffer()
    await pc.setLocalDescription(offer)
    ws.send(JSON.stringify({ type: 'offer', sdp: offer }))
})
