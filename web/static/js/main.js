// Copy code functionality
document.addEventListener('DOMContentLoaded', function() {
  const copyButtons = document.querySelectorAll('.copy-button');

  copyButtons.forEach(button => {
    button.addEventListener('click', async function() {
      const codeBlock = this.nextElementSibling.querySelector('code');
      const code = codeBlock.textContent;

      try {
        await navigator.clipboard.writeText(code);

        // Show success state
        this.classList.add('copied');
        const originalHTML = this.innerHTML;
        this.innerHTML = `
          <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <polyline points="20 6 9 17 4 12"></polyline>
          </svg>
        `;

        // Reset after 2 seconds
        setTimeout(() => {
          this.classList.remove('copied');
          this.innerHTML = originalHTML;
        }, 2000);
      } catch (err) {
        console.error('Failed to copy code:', err);
      }
    });
  });
});
