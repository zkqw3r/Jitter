const params = new URLSearchParams(location.search)
const ROOM_ID = params.get('room')
const ws = new WebSocket(`ws://${location.host}/ws/${ROOM_ID}`)
const pc = new RTCPeerConnection({
    iceServers: [{ urls: 'stun:stun.l.google.com:19302' }]
})
const iceCandidateBuffer = []
let remoteDescriptionSet = false

const streamReady = navigator.mediaDevices.getUserMedia({ video: true, audio: true })
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
    const video = document.getElementById('remoteVideo')
    video.srcObject = streams[0]
    streams[0].getVideoTracks().forEach(track => {
        track.onmute = () => {
            video.style.display = 'none'
            document.getElementById('remoteAvatar').classList.add('visible')
        }
        track.onunmute = () => {
            video.style.display = 'block'
            document.getElementById('remoteAvatar').classList.remove('visible')
        }
    })
}

async function startCall() {
    const offer = await pc.createOffer()
    await pc.setLocalDescription(offer)
    ws.send(JSON.stringify({ type: 'offer', sdp: offer }))
}

async function endCall() {
    const stream = document.getElementById('localVideo').srcObject
    stream.getTracks().forEach(t => t.stop())
    
    pc.close()
    ws.close()
    
    location.href = '/'
}


ws.onmessage = async ({ data }) => {
    const msg = JSON.parse(data)

    if (msg.type === 'offer') {
        await streamReady
        await pc.setRemoteDescription(msg.sdp)
        remoteDescriptionSet = true
        for (const candidate of iceCandidateBuffer) {
            await pc.addIceCandidate(candidate)
        }
        iceCandidateBuffer.length = 0
        const answer = await pc.createAnswer()
        await pc.setLocalDescription(answer)
        ws.send(JSON.stringify({ type: 'answer', sdp: answer }))
    } else if (msg.type === 'answer') {
        await pc.setRemoteDescription(msg.sdp)
        remoteDescriptionSet = true
        for (const candidate of iceCandidateBuffer) {
            await pc.addIceCandidate(candidate)
        }
        iceCandidateBuffer.length = 0
    } else if (msg.type === 'ice') {
        if (remoteDescriptionSet) {
            await pc.addIceCandidate(msg.candidate)
        } else {
            iceCandidateBuffer.push(msg.candidate)
        }
    } else if (msg.type === 'peer-joined') {
        startCall()
    } else if (msg.type === 'cam-off') {
        document.getElementById('remoteVideo').style.display = 'none'
        document.getElementById('remoteAvatar').classList.add('visible')
    } else if (msg.type === 'cam-on') {
        document.getElementById('remoteVideo').style.display = 'block'
        document.getElementById('remoteAvatar').classList.remove('visible')
    }
}

function toggleMic() {
    const track = document.getElementById('localVideo').srcObject.getAudioTracks()[0]
    track.enabled = !track.enabled
    document.getElementById('micBtn').classList.toggle('off')
}

function toggleMute() {
    const video = document.getElementById('remoteVideo')
    video.muted = !video.muted
    document.getElementById('muteBtn').textContent = video.muted ? '🔇' : '🔊'
}

function toggleCam() {
    const track = document.getElementById('localVideo').srcObject.getVideoTracks()[0]
    track.enabled = !track.enabled
    const isOff = !track.enabled
    ws.send(JSON.stringify({ type: isOff ? 'cam-off' : 'cam-on' }))
    document.getElementById('camBtn').classList.toggle('off')
    document.getElementById('localVideo').style.display = isOff ? 'none' : 'block'
    document.getElementById('localAvatar').classList.toggle('visible', isOff)
}

function toggleFullscreen(wrapId) {
    const wrap = document.getElementById(wrapId)
    if (!document.fullscreenElement) {
        wrap.requestFullscreen()
    } else {
        document.exitFullscreen()
    }
}
