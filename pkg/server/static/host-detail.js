
document.querySelectorAll('.latency-graph-tab-button').forEach(button => {
    button.addEventListener('click', function(e) {
        const target = e.target.dataset.target;
        document.querySelectorAll('.latency-graph-tab-button').forEach(button => {
            button.classList.remove('active');
        });
        e.target.classList.add('active'); 
        switchTab(target);
    });
});

function switchTab(target) {
    if (target === 'all') {
        document.querySelectorAll('#latency-graph-panels .data-panel').forEach(panel => {  
            panel.style.display = 'block';
        });
    }
    else {
        document.querySelectorAll('#latency-graph-panels .data-panel').forEach(panel => {  
            panel.style.display = 'none';
        });
        document.querySelector(`#latency-graph-panels .data-panel#panel-${target}`).style.display = 'block';
    }   
}