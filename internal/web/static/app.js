// Zap File Saver - Frontend Logic

document.addEventListener('DOMContentLoaded', () => {
    // UI Elements
    const connectionBadge = document.getElementById('connection-badge');
    const connectionStatusText = document.getElementById('connection-status-text');
    const qrPlaceholder = document.getElementById('qr-placeholder');
    const qrPlaceholderText = document.getElementById('qr-placeholder-text');
    const qrImage = document.getElementById('qr-image');
    const authActions = document.getElementById('auth-actions');
    const btnLogout = document.getElementById('btn-logout');
    
    const configForm = document.getElementById('config-form');
    const geminiKeyInput = document.getElementById('gemini-key');
    const modelSelect = document.getElementById('model-select');
    const groupSelect = document.getElementById('group-select');
    const btnSaveConfig = document.getElementById('btn-save-config');
    const btnRefreshGroups = document.getElementById('btn-refresh-groups');
    
    const logConsole = document.getElementById('log-console');
    const btnClearLogs = document.getElementById('btn-clear-logs');
    
    const filesTableBody = document.getElementById('files-table-body');

    let socket = null;
    let qrActive = false;
    let savedGroupJID = '';
    let savedGeminiModel = 'gemini-3.5-flash';

    // Initialize Page
    loadConfig();
    loadFilesHistory();
    setupWebSocket();

    // Event Listeners
    configForm.addEventListener('submit', saveConfig);
    btnRefreshGroups.addEventListener('click', loadGroups);
    btnLogout.addEventListener('click', handleLogout);
    btnClearLogs.addEventListener('click', () => {
        logConsole.innerHTML = '<div class="log-line log-system">[Sistema] Console limpo.</div>';
    });
    
    // Automatically load models list when API key is entered or blurred
    geminiKeyInput.addEventListener('blur', () => {
        const key = geminiKeyInput.value.trim();
        if (key) {
            loadModels(key);
        }
    });

    // WebSocket setup with automatic reconnect
    function setupWebSocket() {
        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        const wsUrl = `${protocol}//${window.location.host}/ws`;
        
        appendLog('system', 'Conectando ao canal de eventos em tempo real...');
        socket = new WebSocket(wsUrl);

        socket.onopen = () => {
            appendLog('success', 'Conexão estabelecida com o servidor de eventos.');
        };

        socket.onmessage = (event) => {
            try {
                const message = JSON.parse(event.data);
                handleWSMessage(message);
            } catch (err) {
                console.error('Erro ao processar mensagem do WebSocket:', err);
            }
        };

        socket.onclose = () => {
            appendLog('error', 'Conexão WebSocket perdida. Tentando reconectar em 5 segundos...');
            updateStateUI('disconnected');
            setTimeout(setupWebSocket, 5000);
        };

        socket.onerror = (error) => {
            console.error('Erro no WebSocket:', error);
        };
    }

    // Handle incoming WS events
    function handleWSMessage(message) {
        const { type, payload } = message;

        switch (type) {
            case 'state':
                updateStateUI(payload);
                break;
            case 'qr':
                updateQRUI(payload);
                break;
            case 'log':
                // Check format and parse log category
                let logType = 'system';
                if (payload.includes('[WhatsApp]')) logType = 'whatsapp';
                else if (payload.includes('[Gemini]')) logType = 'gemini';
                else if (payload.includes('[Erro]')) logType = 'error';
                else if (payload.includes('[Sucesso]')) logType = 'success';
                
                appendLog(logType, payload);
                break;
            case 'file_processed':
                addFileToTable(payload);
                break;
        }
    }

    // Update UI depending on connection state
    function updateStateUI(state) {
        // Clear classes
        connectionBadge.className = 'badge';
        
        if (state === 'connected') {
            connectionBadge.classList.add('badge-connected');
            connectionStatusText.textContent = 'Conectado';
            
            qrPlaceholder.classList.add('hidden');
            qrImage.classList.add('hidden');
            authActions.classList.remove('hidden');
            
            // Enable config controls
            geminiKeyInput.disabled = false;
            modelSelect.disabled = false;
            groupSelect.disabled = false;
            btnSaveConfig.disabled = false;
            btnRefreshGroups.disabled = false;
            
            // If we just transitioned to connected and haven't loaded groups yet
            if (!qrActive) {
                loadGroups();
            }
            qrActive = false;
        } else if (state === 'connecting') {
            connectionBadge.classList.add('badge-connecting');
            connectionStatusText.textContent = 'Conectando';
            
            if (!qrActive) {
                qrPlaceholder.classList.remove('hidden');
                qrPlaceholderText.textContent = 'Inicializando conexão com o WhatsApp...';
                qrImage.classList.add('hidden');
            }
            authActions.classList.add('hidden');
            
            // Disable config controls
            modelSelect.disabled = true;
            groupSelect.disabled = true;
            btnSaveConfig.disabled = true;
            btnRefreshGroups.disabled = true;
        } else { // disconnected
            connectionBadge.classList.add('badge-disconnected');
            connectionStatusText.textContent = 'Desconectado';
            
            if (!qrActive) {
                qrPlaceholder.classList.remove('hidden');
                qrPlaceholderText.textContent = 'WhatsApp desconectado. Aguardando QR Code...';
                qrImage.classList.add('hidden');
            }
            authActions.classList.add('hidden');
            
            // Disable config controls
            modelSelect.disabled = true;
            groupSelect.disabled = true;
            btnSaveConfig.disabled = true;
            btnRefreshGroups.disabled = true;
            groupSelect.innerHTML = '<option value="">-- Conecte o WhatsApp Primeiro --</option>';
        }
    }

    // Update QR Code display
    function updateQRUI(qrBase64) {
        if (qrBase64) {
            qrActive = true;
            qrImage.src = qrBase64;
            qrImage.classList.remove('hidden');
            qrPlaceholder.classList.add('hidden');
        } else {
            qrActive = false;
            qrImage.classList.add('hidden');
            qrImage.src = '';
        }
    }

    // Load configs from API
    async function loadConfig() {
        try {
            const response = await fetch('/api/config');
            if (response.ok) {
                const config = await response.json();
                geminiKeyInput.value = config.gemini_api_key || '';
                savedGroupJID = config.monitored_group_jid || '';
                savedGeminiModel = config.gemini_model || 'gemini-3.5-flash';
                
                if (config.gemini_api_key) {
                    loadModels(config.gemini_api_key);
                }
            }
        } catch (err) {
            appendLog('error', '[Sistema] Falha ao carregar configurações do servidor.');
        }
    }

    // Save configurations
    async function saveConfig(e) {
        e.preventDefault();
        
        const apiKey = geminiKeyInput.value.trim();
        const selectedOption = groupSelect.options[groupSelect.selectedIndex];
        
        if (!apiKey) {
            alert('A chave de API do Gemini é obrigatória.');
            return;
        }

        const groupJID = groupSelect.value;
        const groupName = groupJID ? selectedOption.text : '';
        const model = modelSelect.value;

        try {
            const response = await fetch('/api/config', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    gemini_api_key: apiKey,
                    gemini_model: model,
                    monitored_group_jid: groupJID,
                    monitored_group_name: groupName
                })
            });

            if (response.ok) {
                savedGroupJID = groupJID;
                appendLog('success', '[Config] Configurações salvas com sucesso!');
                alert('Configurações salvas!');
            } else {
                const errText = await response.text();
                appendLog('error', `[Config] Erro ao salvar configurações: ${errText}`);
            }
        } catch (err) {
            appendLog('error', '[Config] Falha de rede ao tentar salvar configurações.');
        }
    }

    // Load WhatsApp groups list
    async function loadGroups() {
        groupSelect.innerHTML = '<option value="">-- Carregando grupos... --</option>';
        try {
            const response = await fetch('/api/groups');
            if (response.ok) {
                const groups = await response.json();
                
                groupSelect.innerHTML = '<option value="">-- Selecione o grupo para monitoramento --</option>';
                
                if (!groups || groups.length === 0) {
                    groupSelect.innerHTML = '<option value="">Nenhum grupo encontrado</option>';
                    return;
                }

                groups.forEach(group => {
                    const option = document.createElement('option');
                    option.value = group.jid;
                    option.textContent = group.name;
                    if (group.jid === savedGroupJID) {
                        option.selected = true;
                    }
                    groupSelect.appendChild(option);
                });
                
                appendLog('system', `[WhatsApp] ${groups.length} grupos carregados com sucesso.`);
            } else {
                groupSelect.innerHTML = '<option value="">Erro ao carregar grupos</option>';
                appendLog('error', '[WhatsApp] Falha ao carregar lista de grupos.');
            }
        } catch (err) {
            groupSelect.innerHTML = '<option value="">Falha de conexão</option>';
            console.error('Erro ao buscar grupos:', err);
        }
    }

    // Log out WhatsApp session
    async function handleLogout() {
        if (!confirm('Deseja realmente desconectar e esquecer esta sessão do WhatsApp?')) {
            return;
        }

        try {
            const response = await fetch('/api/logout', { method: 'POST' });
            if (response.ok) {
                appendLog('system', '[WhatsApp] Desconexão solicitada. Limpando sessão...');
            } else {
                appendLog('error', '[WhatsApp] Falha ao solicitar desconexão.');
            }
        } catch (err) {
            appendLog('error', '[WhatsApp] Erro de rede ao desconectar.');
        }
    }

    // Load initial file processing logs
    async function loadFilesHistory() {
        try {
            const response = await fetch('/api/logs');
            if (response.ok) {
                const logs = await response.json();
                
                if (logs && logs.length > 0) {
                    filesTableBody.innerHTML = '';
                    logs.forEach(log => addFileToTable(log, false)); // Prepend is handled correctly
                }
            }
        } catch (err) {
            console.error('Erro ao buscar histórico de arquivos:', err);
        }
    }

    // Helper to format date strings from server
    formatDateTime = (timeStr) => {
        try {
            const d = new Date(timeStr);
            if (isNaN(d.getTime())) return timeStr;
            return d.toLocaleString('pt-BR');
        } catch (e) {
            return timeStr;
        }
    };

    // Add row to files history table
    function addFileToTable(fileLog, prepend = true) {
        // Remove empty row helper
        const emptyRow = filesTableBody.querySelector('.empty-row');
        if (emptyRow) {
            filesTableBody.removeChild(emptyRow);
        }

        const tr = document.createElement('tr');
        
        const statusClass = fileLog.status === 'success' ? 'status-success' : 'status-failed';
        const statusText = fileLog.status === 'success' ? '✓ Sucesso' : '✗ Erro';
        const storageDisplay = fileLog.status === 'success' 
            ? `<span class="file-path-badge" title="${fileLog.storage_path}">${fileLog.storage_path}</span>` 
            : `<span class="text-danger">${fileLog.error_message || 'Falha no processamento'}</span>`;

        tr.innerHTML = `
            <td>${formatDateTime(fileLog.timestamp)}</td>
            <td><strong>${escapeHtml(fileLog.sender_name)}</strong></td>
            <td>${escapeHtml(fileLog.original_name)}</td>
            <td><span class="badge-cat badge-cat-${fileLog.category}">${escapeHtml(fileLog.category || '-')}</span></td>
            <td>
                <strong>${escapeHtml(fileLog.new_name || '-')}</strong>
                ${storageDisplay}
            </td>
            <td><span class="status-cell ${statusClass}">${statusText}</span></td>
        `;

        if (prepend) {
            filesTableBody.insertBefore(tr, filesTableBody.firstChild);
        } else {
            filesTableBody.appendChild(tr);
        }
    }

    // Helper to append log messages to terminal console
    function appendLog(type, message) {
        const line = document.createElement('div');
        line.className = `log-line log-${type}`;
        
        const timestamp = new Date().toLocaleTimeString('pt-BR');
        line.innerHTML = `<span class="log-time">[${timestamp}]</span> ${escapeHtml(message)}`;
        
        logConsole.appendChild(line);
        logConsole.scrollTop = logConsole.scrollHeight;
    }

    // Safe HTML Escaping helper
    function escapeHtml(unsafe) {
        if (!unsafe) return '';
        return unsafe
            .replace(/&/g, "&amp;")
            .replace(/</g, "&lt;")
            .replace(/>/g, "&gt;")
            .replace(/"/g, "&quot;")
            .replace(/'/g, "&#039;");
    }

    // Load available Gemini models list
    async function loadModels(customKey = '') {
        const queryParam = customKey ? `?key=${encodeURIComponent(customKey)}` : '';
        try {
            const response = await fetch(`/api/models${queryParam}`);
            if (response.ok) {
                const models = await response.json();
                
                modelSelect.innerHTML = '';
                
                if (!models || models.length === 0) {
                    modelSelect.innerHTML = '<option value="gemini-3.5-flash">gemini-3.5-flash (Padrão)</option>';
                    return;
                }

                models.forEach(model => {
                    const option = document.createElement('option');
                    option.value = model.name;
                    option.textContent = `${model.display_name} (${model.name})`;
                    if (model.name === savedGeminiModel) {
                        option.selected = true;
                    }
                    modelSelect.appendChild(option);
                });
                
                // If the saved model isn't in the list (or it's empty), ensure we have a fallback
                if (modelSelect.value === '' && savedGeminiModel) {
                    const option = document.createElement('option');
                    option.value = savedGeminiModel;
                    option.textContent = savedGeminiModel;
                    option.selected = true;
                    modelSelect.appendChild(option);
                }
            } else {
                appendLog('error', '[Gemini] Falha ao carregar lista de modelos. Verifique se a chave de API é válida.');
            }
        } catch (err) {
            console.error('Erro ao buscar modelos:', err);
        }
    }
});
