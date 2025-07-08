document.addEventListener('DOMContentLoaded', () => {
    const form = document.getElementById('config-form');
    const runTestBtn = document.getElementById('run-test');
    const saveConfigBtn = document.getElementById('save-config');
    const progressLogContainer = document.getElementById('progress-log');
    const progressLog = progressLogContainer.querySelector('pre');
    const resultsPanel = document.getElementById('results-panel');
    const resultsTableContainer = document.getElementById('results-table-container');
    const copyAllBtn = document.getElementById('copy-all-ips');


    let currentConfig = {};
    let locationsData = {};

    // --- WebSocket Logic ---
    let socket;

    function connectWebSocket() {
        socket = new WebSocket(`ws://${window.location.host}/ws/run`);

        socket.onopen = () => {
            console.log('WebSocket connected');
            appendLog('WebSocket 已连接，准备开始测试...');
        };

        socket.onmessage = (event) => {
            try {
                const message = JSON.parse(event.data);
                if (message.type === 'log') {
                    appendLog(message.payload);
                } else if (message.type === 'result') {
                    displayResults(message.payload);
                }
            } catch (e) {
                console.error('Failed to parse WebSocket message:', e);
                appendLog(`错误: 收到无法解析的消息: ${event.data}`);
            }
        };

        const onTestEnd = () => {
            runTestBtn.disabled = false;
            runTestBtn.innerHTML = '<span class="icon">▶️</span> 单次测速';
        };

        socket.onclose = () => {
            console.log('WebSocket disconnected');
            appendLog('WebSocket 已断开。');
            onTestEnd();
        };

        socket.onerror = (error) => {
            console.error('WebSocket error:', error);
            appendLog(`\nWebSocket 错误: ${error}\n`);
            onTestEnd();
        };
    }

    // --- UI Generation ---
    function createFormGroup(id, label, type = 'number', options = {}) {
        const group = document.createElement('div');
        group.className = 'form-group';

        const labelEl = document.createElement('label');
        labelEl.setAttribute('for', id);
        labelEl.textContent = label;
        group.appendChild(labelEl);

        if (type === 'select') {
            const selectEl = document.createElement('select');
            selectEl.id = id;
            options.choices.forEach(choice => {
                const optionEl = document.createElement('option');
                optionEl.value = choice.value;
                optionEl.textContent = choice.text;
                selectEl.appendChild(optionEl);
            });
            group.appendChild(selectEl);
        } else if (type === 'tags') {
             const container = document.createElement('div');
             container.id = id;
             container.className = 'tag-container';
             group.appendChild(container);
        } else {
            const inputEl = document.createElement('input');
            inputEl.type = type;
            inputEl.id = id;
            if (options.disabled) {
                inputEl.disabled = true;
            }
            group.appendChild(inputEl);
            // Add listener specifically for number inputs in the editable form
            if (type === 'number' && !options.disabled) {
                inputEl.addEventListener('input', validateFormAndApplyUI);
            }
        }
        
        if (options.description) {
            const descEl = document.createElement('p');
            descEl.className = 'description';
            descEl.textContent = options.description;
            group.appendChild(descEl);
        }

        return group;
    }

    function renderForm(config, locations) {
        const editableForm = document.getElementById('editable-config-form');
        const nonEditableForm = document.getElementById('non-editable-config-form');
        editableForm.innerHTML = '';
        nonEditableForm.innerHTML = '';

        // Non-Editable Concurrency settings
        nonEditableForm.appendChild(createFormGroup('dns_concurrency', 'DNS 并发数', 'number', { disabled: true, description: '高并发设置，通常无需修改。' }));
        nonEditableForm.appendChild(createFormGroup('latency_test_concurrency', '延迟测试并发数', 'number', { disabled: true, description: '高并发设置，通常无需修改。' }));
        nonEditableForm.appendChild(createFormGroup('speedtest_concurrency', '速度测试并发数', 'number', { disabled: true, description: '高并发设置，通常无需修改。' }));

        // Editable settings
        editableForm.appendChild(createFormGroup('max_latency', '最大延迟 (ms)'));
        editableForm.appendChild(createFormGroup('speedtest_rate_limit_mb', '速度上限 (MB/s, 0为不限速)'));
        editableForm.appendChild(createFormGroup('min_speed', '最小速度 (MB/s, 0为不限速)'));
        editableForm.appendChild(createFormGroup('ip_version', 'IP 版本', 'select', { choices: [{value: 'ipv4', text: 'IPv4'}, {value: 'ipv6', text: 'IPv6'}] }));
        editableForm.appendChild(createFormGroup('group_by', '分组方式', 'select', { choices: [{value: 'region', text: '按地理区域'}, {value: 'colo', text: '按数据中心'}] }));
        
        // Tag-based filters
        editableForm.appendChild(createFormGroup('filter_regions', '筛选区域 (留空则全选)', 'tags'));
        editableForm.appendChild(createFormGroup('filter_colos', '筛选数据中心 (留空则全选)', 'tags'));

        // Add top_n_per_group at the end for better logical flow
        editableForm.appendChild(createFormGroup('top_n_per_group', '每组优选 IP 数'));

        // Populate form with config values
        for (const key in config) {
            const el = document.getElementById(key);
            if (el && el.type !== 'tags') {
                el.value = config[key];
            }
        }
        
        // Populate tags
        populateTags('filter_regions', locations.Regions, config.filter_regions || []);
        populateTags('filter_colos', locations.Colos, config.filter_colos || []);
    }
    
    function populateTags(containerId, allTagsWithOptions, selectedTags) {
        const container = document.getElementById(containerId);
        container.innerHTML = '';
        
        // Filter for common tags and sort them
        const commonTags = allTagsWithOptions.filter(t => t.is_common);
        commonTags.sort((a, b) => a.display.localeCompare(b.display));

        commonTags.forEach(tagInfo => {
            const btn = document.createElement('button');
            btn.className = 'tag-btn';
            btn.textContent = tagInfo.display; // Show Chinese name
            btn.dataset.tag = tagInfo.key;      // Store English key
            
            if (selectedTags.includes(tagInfo.key)) {
                btn.classList.add('selected');
            }
            
            btn.addEventListener('click', (e) => {
                e.preventDefault();
                btn.classList.toggle('selected');
            });
            container.appendChild(btn);
        });
    }

    function getFormData() {
        const config = {};
        const formElements = document.querySelectorAll('#editable-config-form input, #editable-config-form select');
        formElements.forEach(el => {
            if (!el.disabled) {
                 if (el.type === 'number') {
                    config[el.id] = parseFloat(el.value) || 0;
                } else {
                    config[el.id] = el.value;
                }
            }
        });
        
        // Get selected tags
        config.filter_regions = Array.from(document.querySelectorAll('#filter_regions .tag-btn.selected')).map(btn => btn.dataset.tag);
        config.filter_colos = Array.from(document.querySelectorAll('#filter_colos .tag-btn.selected')).map(btn => btn.dataset.tag);

        return config;
    }


    // --- API Calls ---
    async function loadInitialData() {
        try {
            const [configRes, locationsRes] = await Promise.all([
                fetch('/api/config'),
                fetch('/api/locations')
            ]);
            currentConfig = await configRes.json();
            locationsData = await locationsRes.json();
            renderForm(currentConfig, locationsData);
        } catch (error) {
            console.error('Failed to load initial data:', error);
            alert('加载初始配置失败，请检查程序日志。');
        }
    }

    // --- Event Listeners ---
    runTestBtn.addEventListener('click', () => {
        if (!validateFormAndApplyUI()) {
            alert('配置中存在无效值，请修正后再试。');
            return;
        }
        runTestBtn.disabled = true;
        runTestBtn.innerHTML = '<span class="icon">⏳</span> 测试中...';

        progressLog.textContent = ''; // Clear log on new run
        resultsPanel.style.display = 'none'; // Hide previous results
        connectWebSocket();
        // Use a short timeout to ensure socket is ready before sending
        setTimeout(() => {
            if (socket && socket.readyState === WebSocket.OPEN) {
                const configForTest = getFormData();
                socket.send(JSON.stringify(configForTest));
            } else {
                 appendLog('WebSocket 连接失败，无法开始测试。');
                 // The onTestEnd function will be called by the onerror handler,
                 // so no need to manually re-enable the button here.
            }
        }, 500);
    });

    saveConfigBtn.addEventListener('click', async () => {
        if (!validateFormAndApplyUI()) {
            alert('配置中存在无效值，请修正后再试。');
            return;
        }
        if (!confirm('这将覆盖您现有的 config.yaml 文件，确定要继续吗？')) {
            return;
        }
        try {
            const newConfig = getFormData();
            const response = await fetch('/api/config', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(newConfig)
            });
            if (response.ok) {
                alert('配置已成功保存！');
                currentConfig = newConfig; // Update local state
            } else {
                const errorText = await response.text();
                throw new Error(errorText);
            }
        } catch (error) {
            console.error('Failed to save config:', error);
            alert(`保存配置失败: ${error.message}`);
        }
    });

    // --- Initial Load ---
    loadInitialData();

    // --- Helper Functions ---
    function validateFormAndApplyUI() {
        let isOverallValid = true;
        const formElements = document.querySelectorAll('#editable-config-form input[type="number"]');

        // Clear all previous validation states first
        formElements.forEach(el => {
            el.classList.remove('invalid');
            const formGroup = el.closest('.form-group');
            formGroup.querySelectorAll('.error-message').forEach(err => err.remove());
        });

        // Rule 1: Individual field validation (must be a non-negative number)
        formElements.forEach(el => {
            const value = el.value;
            if (value.trim() !== '') { // Only validate if not empty
                const numValue = parseFloat(value);
                if (isNaN(numValue) || numValue < 0) {
                    isOverallValid = false;
                    el.classList.add('invalid');
                    const errorEl = document.createElement('p');
                    errorEl.className = 'error-message';
                    errorEl.textContent = '请输入一个有效的非负数。';
                    el.closest('.form-group').appendChild(errorEl);
                }
            }
        });

        // Rule 2: Cross-field validation (min_speed vs speedtest_rate_limit_mb)
        const minSpeedEl = document.getElementById('min_speed');
        const maxSpeedEl = document.getElementById('speedtest_rate_limit_mb');
        const minSpeed = parseFloat(minSpeedEl.value) || 0;
        const maxSpeed = parseFloat(maxSpeedEl.value) || 0;

        if (maxSpeed > 0 && minSpeed > maxSpeed) {
            isOverallValid = false;
            minSpeedEl.classList.add('invalid');
            maxSpeedEl.classList.add('invalid');
            
            const formGroup = minSpeedEl.closest('.form-group');
            // Avoid adding duplicate messages if one already exists from Rule 1
            if (!formGroup.querySelector('.error-message')) {
                 const errorEl = document.createElement('p');
                 errorEl.className = 'error-message';
                 errorEl.textContent = '最小速度不能大于速度上限。';
                 formGroup.appendChild(errorEl);
            }
        }

        return isOverallValid;
    }

    function appendLog(text) {
        progressLog.textContent += `${text}\n`;
        // Correctly scroll the container, not the pre element
        progressLogContainer.scrollTop = progressLogContainer.scrollHeight;
    }

    function displayResults(results) {
        resultsPanel.style.display = 'block';
        resultsTableContainer.innerHTML = ''; // Clear previous table

        if (!results || results.length === 0) {
            resultsTableContainer.innerHTML = '<p>没有找到符合条件的 IP。</p>';
            return;
        }

        const table = document.createElement('table');
        table.className = 'results-table';

        // Header
        const thead = table.createTHead();
        const headerRow = thead.insertRow();
        const headers = ['IP 地址', '延迟 (ms)', '下载速度 (MB/s)', '数据中心', '地理区域', '操作'];
        headers.forEach(text => {
            const th = document.createElement('th');
            th.textContent = text;
            headerRow.appendChild(th);
        });

        // Body
        const tbody = table.createTBody();
        results.forEach(res => {
            const row = tbody.insertRow();
            row.insertCell().textContent = res.Address;
            row.insertCell().textContent = (res.Delay / 1000000).toFixed(2); // 纳秒转毫秒
            row.insertCell().textContent = (res.DownloadSpeed / 1024).toFixed(2); // KB/s to MB/s
            row.insertCell().textContent = res.Colo;
            row.insertCell().textContent = res.Region;
            
            const actionCell = row.insertCell();
            const copyBtn = document.createElement('button');
            copyBtn.textContent = '复制';
            copyBtn.className = 'copy-ip-btn';
            copyBtn.addEventListener('click', () => copyToClipboard(res.Address, copyBtn));
            actionCell.appendChild(copyBtn);
        });

        resultsTableContainer.appendChild(table);

        // Scroll to results panel
        resultsPanel.scrollIntoView({ behavior: 'smooth', block: 'start' });
    }

    function copyToClipboard(text, buttonElement) {
        navigator.clipboard.writeText(text).then(() => {
            const originalText = buttonElement.textContent;
            buttonElement.textContent = '已复制!';
            setTimeout(() => {
                buttonElement.textContent = originalText;
            }, 1500);
        }).catch(err => {
            console.error('Failed to copy text: ', err);
            alert('复制失败!');
        });
    }

    copyAllBtn.addEventListener('click', () => {
        const allIps = Array.from(document.querySelectorAll('.results-table tbody tr'))
            .map(row => row.cells[0].textContent)
            .join('\n');
        
        if (allIps) {
            copyToClipboard(allIps, copyAllBtn);
        } else {
            alert('没有可复制的 IP。');
        }
    });
});