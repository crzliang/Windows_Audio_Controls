const POLL_INTERVAL = 2000;
let isInteracting = false;
let updateTimer = null;

const state = {
    master: { vol: 50, mute: false },
    media: { isPlaying: false },
    sessions: [] // { pid, name, volume, mute }
};

const i18n = {
    zh: {
        title: "音量控制",
        header: "音频控制",
        subtitle: "Windows Web 控制器",
        master_vol: "主音量",
        apps_title: "应用音量",
        media_title: "播放控制",
        loading: "正在读取音频应用...",
        empty: "当前没有发声的应用",
        langToggle: "EN"
    },
    en: {
        title: "Volume Control",
        header: "Audio Control",
        subtitle: "Windows Web Controller",
        master_vol: "Master Volume",
        apps_title: "Applications",
        media_title: "Playback",
        loading: "Loading audio sessions...",
        empty: "No active audio applications found.",
        langToggle: "中"
    }
};

let currentLang = localStorage.getItem('lang') || 'zh';

// Elements
const masterSlider = document.getElementById('master-slider');
const masterVal = document.getElementById('master-val');
const masterMuteBtn = document.getElementById('master-mute-btn');
const masterIconVol = document.getElementById('master-icon-vol');
const masterIconMute = document.getElementById('master-icon-mute');
const sessionsContainer = document.getElementById('sessions-container');
const sessionTemplate = document.getElementById('session-template');
const langBtn = document.getElementById('lang-btn');

function applyLang() {
    document.documentElement.lang = currentLang;
    const txts = i18n[currentLang];
    document.querySelectorAll('[data-i18n]').forEach(el => {
        const key = el.getAttribute('data-i18n');
        if (txts[key]) {
            if (el.tagName === 'TITLE') document.title = txts[key];
            else el.innerText = txts[key];
        }
    });
    langBtn.innerText = txts.langToggle;
    
    // Refresh empty state inner text if it exists
    if (state.sessions.length === 0 && sessionsContainer.querySelector('.loading-state')) {
        sessionsContainer.innerHTML = `<div class="loading-state" data-i18n="empty">${txts.empty}</div>`;
    }
}

// Initialize
async function init() {
    applyLang();
    
    langBtn.addEventListener('click', () => {
        currentLang = currentLang === 'zh' ? 'en' : 'zh';
        localStorage.setItem('lang', currentLang);
        applyLang();
    });

    await fetchStatus();
    renderMaster();
    renderSessions();
    setupEventListeners();
    setupMediaEventListeners();
    
    // Poll loop
    setInterval(async () => {
        if (!isInteracting) {
            const sessionsChanged = await fetchStatus();
            if(!isInteracting) {
                renderMaster();
                renderMedia();
                if(sessionsChanged) {
                    renderSessions();
                } else {
                    updateSessionDOMValues();
                }
            }
        }
    }, POLL_INTERVAL);
}

// Data Fetching
async function fetchStatus() {
    try {
        const res = await fetch('/api/status');
        const data = await res.json();
        
        state.master.vol = Math.round(data.masterVolume * 100);
        state.master.mute = data.masterMute;
        
        if (data.media) {
            state.media.isPlaying = data.media.status === 'Playing' || data.media.status === 4;
        } else {
            state.media.isPlaying = false;
        }

        let changed = false;
        if (state.sessions.length !== data.sessions.length) changed = true;
        
        const newSessions = data.sessions.map(s => ({
            pid: s.pid,
            name: s.name,
            vol: Math.round(s.volume * 100),
            mute: s.mute
        }));
        
        if(!changed) {
            for(let i=0; i<newSessions.length; i++) {
                if(newSessions[i].pid !== state.sessions[i].pid || newSessions[i].name !== state.sessions[i].name) {
                    changed = true; break;
                }
            }
        }
        
        if (changed) {
            const currentPids = newSessions.map(s => s.pid);
            Object.keys(timers).forEach(pidStr => {
                if (pidStr !== 'master') {
                    const p = parseInt(pidStr, 10);
                    if (!isNaN(p) && !currentPids.includes(p)) {
                        clearTimeout(timers[pidStr]);
                        delete timers[pidStr];
                    }
                }
            });
        }
        
        state.sessions = newSessions;
        return changed;

    } catch (e) {
        console.error("Fetch status failed:", e);
        return false;
    }
}

function renderMedia() {
    const playPauseBtn = document.getElementById('media-play-pause');
    const playIcon = playPauseBtn.querySelector('.play-icon');
    const pauseIcon = playPauseBtn.querySelector('.pause-icon');

    if (state.media.isPlaying) {
        playIcon.style.display = 'none';
        pauseIcon.style.display = 'flex';
    } else {
        playIcon.style.display = 'flex';
        pauseIcon.style.display = 'none';
    }
}

function updateSliderBackground(slider, val, isMaster = false) {
    const color = isMaster ? 'var(--accent)' : '#64748b';
    slider.style.background = `linear-gradient(to right, ${color} 0%, ${color} ${val}%, var(--slider-track) ${val}%, var(--slider-track) 100%)`;
}

// Render Master
function renderMaster() {
    masterSlider.value = state.master.vol;
    masterVal.innerText = `${state.master.vol}%`;
    updateSliderBackground(masterSlider, state.master.vol, true);
    
    if (state.master.mute) {
        masterMuteBtn.classList.add('is-muted');
        masterIconVol.style.display = 'none';
        masterIconMute.style.display = 'block';
    } else {
        masterMuteBtn.classList.remove('is-muted');
        masterIconVol.style.display = 'block';
        masterIconMute.style.display = 'none';
    }
}

function getAppInitials(name) {
    if(!name) return '?';
    let clean = name.replace('.exe', '');
    return clean.charAt(0).toUpperCase();
}

// Render Sessions
function renderSessions() {
    sessionsContainer.innerHTML = '';
    
    if (state.sessions.length === 0) {
        sessionsContainer.innerHTML = `<div class="loading-state" data-i18n="empty">${i18n[currentLang].empty}</div>`;
        return;
    }

    state.sessions.forEach(session => {
        const clone = sessionTemplate.content.cloneNode(true);
        const card = clone.querySelector('.session-card');
        card.dataset.pid = session.pid;
        
        clone.querySelector('.app-name').innerText = session.name.replace('.exe', '');
        clone.querySelector('.app-pid').innerText = `PID: ${session.pid}`;
        clone.querySelector('.app-icon').innerText = getAppInitials(session.name);
        
        const slider = clone.querySelector('.app-slider');
        const valText = clone.querySelector('.app-val-text');
        const muteBtn = clone.querySelector('.mute-btn');
        
        slider.value = session.vol;
        valText.innerText = `${session.vol}%`;
        updateSliderBackground(slider, session.vol, false);
        
        if (session.mute) {
            muteBtn.classList.add('is-muted');
            muteBtn.querySelector('.icon-vol').style.display = 'none';
            muteBtn.querySelector('.icon-mute').style.display = 'block';
        }
        
        // Events
        slider.addEventListener('input', (e) => {
            isInteracting = true;
            valText.innerText = `${e.target.value}%`;
            updateSliderBackground(slider, e.target.value, false);
            debouncedUpdateSession(session.pid, e.target.value / 100);
        });
        
        slider.addEventListener('change', () => setTimeout(() => isInteracting = false, 1000));
        slider.addEventListener('touchstart', () => isInteracting = true, {passive: true});
        slider.addEventListener('touchend', () => setTimeout(() => isInteracting = false, 1000), {passive: true});
        
        muteBtn.addEventListener('click', async () => {
             const newMute = !session.mute;
             session.mute = newMute;
             if (newMute) {
                 muteBtn.classList.add('is-muted');
                 muteBtn.querySelector('.icon-vol').style.display = 'none';
                 muteBtn.querySelector('.icon-mute').style.display = 'block';
             } else {
                 muteBtn.classList.remove('is-muted');
                 muteBtn.querySelector('.icon-vol').style.display = 'block';
                 muteBtn.querySelector('.icon-mute').style.display = 'none';
             }
             await fetch(`/api/sessions/volume?pid=${session.pid}&mute=${newMute}`, { method: 'POST' });
        });

        sessionsContainer.appendChild(clone);
    });
}

function updateSessionDOMValues() {
    state.sessions.forEach(session => {
        const card = document.querySelector(`.session-card[data-pid="${session.pid}"]`);
        if(card) {
            const slider = card.querySelector('.app-slider');
            const valText = card.querySelector('.app-val-text');
            const muteBtn = card.querySelector('.mute-btn');
            
            slider.value = session.vol;
            valText.innerText = `${session.vol}%`;
            updateSliderBackground(slider, session.vol, false);
            
            if (session.mute) {
                 muteBtn.classList.add('is-muted');
                 muteBtn.querySelector('.icon-vol').style.display = 'none';
                 muteBtn.querySelector('.icon-mute').style.display = 'block';
             } else {
                 muteBtn.classList.remove('is-muted');
                 muteBtn.querySelector('.icon-vol').style.display = 'block';
                 muteBtn.querySelector('.icon-mute').style.display = 'none';
             }
        }
    });
}

function setupEventListeners() {
    masterSlider.addEventListener('input', (e) => {
        isInteracting = true;
        masterVal.innerText = `${e.target.value}%`;
        updateSliderBackground(masterSlider, e.target.value, true);
        debouncedUpdateMaster(e.target.value / 100);
    });
    
    masterSlider.addEventListener('change', () => setTimeout(() => isInteracting = false, 1000));
    masterSlider.addEventListener('touchstart', () => isInteracting = true, {passive: true});
    masterSlider.addEventListener('touchend', () => setTimeout(() => isInteracting = false, 1000), {passive: true});

    masterMuteBtn.addEventListener('click', async () => {
        const newMute = !state.master.mute;
        state.master.mute = newMute;
        renderMaster();
        await fetch(`/api/master/volume?mute=${newMute}`, { method: 'POST' });
    });
}

function setupMediaEventListeners() {
    const prevBtn = document.getElementById('media-prev');
    const playPauseBtn = document.getElementById('media-play-pause');
    const nextBtn = document.getElementById('media-next');

    prevBtn?.addEventListener('click', () => {
        fetch('/api/media?action=prev', { method: 'POST' }).catch(console.error);
    });

    playPauseBtn?.addEventListener('click', () => {
        fetch('/api/media?action=play_pause', { method: 'POST' }).catch(console.error);
    });

    nextBtn?.addEventListener('click', () => {
        fetch('/api/media?action=next', { method: 'POST' }).catch(console.error);
    });
}

let timers = {};
function debouncedUpdateMaster(level) {
    if(timers['master']) clearTimeout(timers['master']);
    timers['master'] = setTimeout(() => {
        fetch(`/api/master/volume?level=${level}`, { method: 'POST' }).catch(console.error);
    }, 100);
}

function debouncedUpdateSession(pid, level) {
    if(timers[pid]) clearTimeout(timers[pid]);
    timers[pid] = setTimeout(() => {
        fetch(`/api/sessions/volume?pid=${pid}&level=${level}`, { method: 'POST' }).catch(console.error);
    }, 100);
}

window.onload = init;
