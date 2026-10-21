<script lang="ts">
  import { FxaLogin, FxaSubmit2FACode, FxaSignOut } from '../../wailsjs/go/main/App.js';

  export let show = false;
  export let isSignedIn = false;
  export let userEmail = '';
  export let isUnlimited = false;
  export let quotaRemainingBytes = 0;
  export let quotaLimitBytes = 0;
  export let onClose: () => void;
  export let onAuthSuccess: () => void;
  export let onSignOutSuccess: () => void;

  let email = '';
  let password = '';
  let twoFactorCode = '';
  let needs2FA = false;
  let loading = false;
  let errorMessage = '';

  let showPassword = false;
  let showSignOutConfirm = false;

  $: if (show) {
    showSignOutConfirm = false;
    showPassword = false;
    errorMessage = '';
  }

  function focusTrap(node: HTMLElement) {
    const focusableSelectors = 'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])';
    function handleKeyDown(e: KeyboardEvent) {
      if (e.key !== 'Tab') return;
      const focusable = Array.from(node.querySelectorAll<HTMLElement>(focusableSelectors)).filter(
        el => !el.hasAttribute('disabled') && el.offsetParent !== null
      );
      if (focusable.length === 0) return;
      const first = focusable[0];
      const last = focusable[focusable.length - 1];
      if (e.shiftKey) {
        if (document.activeElement === first) {
          last.focus();
          e.preventDefault();
        }
      } else {
        if (document.activeElement === last) {
          first.focus();
          e.preventDefault();
        }
      }
    }
    node.addEventListener('keydown', handleKeyDown);
    return {
      destroy() {
        node.removeEventListener('keydown', handleKeyDown);
      }
    };
  }

  function formatBytes(bytes: number): string {
    if (!bytes || bytes <= 0) return '0 B';
    const units = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(1024));
    return (bytes / Math.pow(1024, i)).toFixed(1) + ' ' + units[i];
  }

  function handleBackToLogin() {
    needs2FA = false;
    twoFactorCode = '';
    errorMessage = '';
  }

  async function handleLogin() {
    errorMessage = '';
    loading = true;
    try {
      if (needs2FA) {
        await FxaSubmit2FACode(twoFactorCode);
        needs2FA = false;
        twoFactorCode = '';
        onAuthSuccess();
        onClose();
      } else {
        const req2FA = await FxaLogin(email, password);
        if (req2FA) {
          needs2FA = true;
        } else {
          password = '';
          onAuthSuccess();
          onClose();
        }
      }
    } catch (e: any) {
      errorMessage = e?.message || String(e);
    } finally {
      loading = false;
    }
  }

  async function handleSignOut() {
    loading = true;
    try {
      await FxaSignOut();
      onSignOutSuccess();
      onClose();
    } catch (e: any) {
      errorMessage = e?.message || String(e);
    } finally {
      loading = false;
      showSignOutConfirm = false;
    }
  }
</script>

{#if show}
  <div 
    class="modal-overlay" 
    role="dialog" 
    aria-modal="true" 
    tabindex="-1"
    on:click={(e) => { if (e.target === e.currentTarget) onClose(); }}
    on:keydown={(e) => { if (e.key === 'Escape') onClose(); }}
  >
    <div class="modal-box" role="document" use:focusTrap>
      <div class="modal-header">
        <h3>Firefox Account</h3>
        <button class="close-btn" aria-label="Close" on:click={onClose}>✕</button>
      </div>

      <div class="modal-body">
        {#if isSignedIn}
          <div class="profile-card">
            <div class="avatar-ring">
              <span class="avatar-letter">{userEmail ? userEmail[0].toUpperCase() : 'F'}</span>
            </div>
            <div class="profile-info">
              <span class="profile-email">{userEmail || 'Active Session'}</span>
              <span class="tier-badge" class:unlimited={isUnlimited}>
                {isUnlimited ? '★ Unlimited VPN Plan' : 'Free 50 GB Monthly Tier'}
              </span>
            </div>
          </div>

          <div class="quota-summary">
            <div class="quota-row">
              <span class="q-label">Available Bandwidth:</span>
              <span class="q-val">
                {isUnlimited ? 'Unlimited' : `${formatBytes(quotaRemainingBytes)} remaining`}
              </span>
            </div>
            {#if !isUnlimited && quotaLimitBytes > 0}
              <div class="q-bar">
                <div class="q-fill" style="width: {Math.min(100, (quotaRemainingBytes / quotaLimitBytes) * 100)}%"></div>
              </div>
            {/if}
          </div>

          {#if showSignOutConfirm}
            <div class="signout-confirm-box">
              <span class="confirm-msg">Sign out of Firefox Account?</span>
              <div class="confirm-actions">
                <button class="btn-danger-confirm" on:click={handleSignOut} disabled={loading}>
                  {loading ? 'Signing out...' : 'Yes, Sign Out'}
                </button>
                <button class="btn-cancel-inline" on:click={() => (showSignOutConfirm = false)}>
                  Cancel
                </button>
              </div>
            </div>
          {:else}
            <button class="btn-signout" on:click={() => (showSignOutConfirm = true)}>
              Sign Out of Firefox Account
            </button>
          {/if}
        {:else}
          <div class="auth-intro">
            <p>Sign in with your Firefox Account to acquire monthly subscription-free Guardian VPN proxy passes.</p>
          </div>

          {#if errorMessage}
            <div class="error-banner">{errorMessage}</div>
          {/if}

          {#if !needs2FA}
            <label class="form-group">
              <span class="form-label">Firefox Account Email</span>
              <input type="email" placeholder="name@example.com" bind:value={email} />
            </label>

            <div class="form-group">
              <span class="form-label">Password</span>
              <div class="password-input-wrapper">
                {#if showPassword}
                  <input type="text" placeholder="••••••••••••" bind:value={password} />
                {:else}
                  <input type="password" placeholder="••••••••••••" bind:value={password} />
                {/if}
                <button 
                  type="button" 
                  class="toggle-pwd-btn" 
                  aria-label={showPassword ? 'Hide password' : 'Show password'}
                  on:click={() => (showPassword = !showPassword)}
                >
                  {#if showPassword}
                    <!-- Eye Off SVG -->
                    <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                      <path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19m-6.72-1.07a3 3 0 1 1-4.24-4.24"></path>
                      <line x1="1" y1="1" x2="23" y2="23"></line>
                    </svg>
                  {:else}
                    <!-- Eye SVG -->
                    <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                      <path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"></path>
                      <circle cx="12" cy="12" r="3"></circle>
                    </svg>
                  {/if}
                </button>
              </div>
            </div>
          {:else}
            <label class="form-group">
              <span class="form-label">Two-Factor Verification Code (2FA)</span>
              <input type="text" placeholder="Enter 6-digit code or backup code" bind:value={twoFactorCode} />
              <span class="hint">Check your authenticator app or email for code</span>
            </label>
            <button type="button" class="btn-back-link" on:click={handleBackToLogin}>
              ← Back to Email & Password
            </button>
          {/if}

          <button class="btn-submit" on:click={handleLogin} disabled={loading}>
            {#if loading}
              <span class="spinner"></span> Signing in...
            {:else}
              {needs2FA ? 'Confirm 2FA Code' : 'Sign In'}
            {/if}
          </button>
        {/if}
      </div>
    </div>
  </div>
{/if}

<style>
  .modal-overlay {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.75);
    backdrop-filter: blur(8px);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 1000;
  }
  .modal-box {
    width: 90%;
    max-width: 420px;
    max-height: 85vh;
    display: flex;
    flex-direction: column;
    background: #0f121d;
    border: 1px solid rgba(255, 255, 255, 0.1);
    border-radius: 16px;
    overflow: hidden;
    box-shadow: 0 20px 50px rgba(0, 0, 0, 0.6);
  }
  .modal-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 16px 20px;
    border-bottom: 1px solid rgba(255, 255, 255, 0.08);
    flex-shrink: 0;
  }
  .modal-header h3 {
    margin: 0;
    font-size: 1.05rem;
    font-weight: 700;
    color: white;
  }
  .close-btn {
    background: transparent;
    border: none;
    color: var(--text-dim);
    font-size: 1.1rem;
    cursor: pointer;
    padding: 4px;
    border-radius: 6px;
  }
  .close-btn:hover {
    color: white;
    background: rgba(255, 255, 255, 0.08);
  }
  .modal-body {
    padding: 20px;
    display: flex;
    flex-direction: column;
    gap: 16px;
    overflow-y: auto;
    min-height: 0;
  }
  .profile-card {
    display: flex;
    align-items: center;
    gap: 14px;
    padding: 14px;
    background: rgba(255, 255, 255, 0.03);
    border: 1px solid rgba(255, 255, 255, 0.08);
    border-radius: 12px;
  }
  .avatar-ring {
    width: 44px;
    height: 44px;
    border-radius: 50%;
    background: linear-gradient(135deg, var(--fox-primary), var(--fox-secondary));
    display: flex;
    align-items: center;
    justify-content: center;
  }
  .avatar-letter {
    color: white;
    font-weight: 700;
    font-size: 1.2rem;
  }
  .profile-info {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }
  .profile-email {
    font-size: 0.9rem;
    font-weight: 600;
    color: white;
  }
  .tier-badge {
    font-size: 0.7rem;
    color: var(--fox-primary);
    font-weight: 600;
  }
  .tier-badge.unlimited {
    color: #10b981;
  }
  .quota-summary {
    display: flex;
    flex-direction: column;
    gap: 8px;
    padding: 14px;
    background: rgba(255, 255, 255, 0.03);
    border-radius: 12px;
  }
  .quota-row {
    display: flex;
    justify-content: space-between;
    font-size: 0.78rem;
  }
  .q-label {
    color: var(--text-muted);
  }
  .q-val {
    color: white;
    font-weight: 600;
  }
  .q-bar {
    height: 6px;
    background: rgba(255, 255, 255, 0.08);
    border-radius: 3px;
    overflow: hidden;
  }
  .q-fill {
    height: 100%;
    background: linear-gradient(90deg, var(--fox-primary), #10b981);
  }
  .btn-signout {
    background: rgba(239, 68, 68, 0.15);
    border: 1px solid rgba(239, 68, 68, 0.3);
    color: #ef4444;
    padding: 10px;
    border-radius: 8px;
    font-size: 0.82rem;
    font-weight: 600;
    cursor: pointer;
  }
  .btn-signout:hover {
    background: rgba(239, 68, 68, 0.25);
  }
  .auth-intro p {
    font-size: 0.8rem;
    color: var(--text-muted);
    line-height: 1.4;
  }
  .error-banner {
    background: rgba(239, 68, 68, 0.15);
    border: 1px solid rgba(239, 68, 68, 0.3);
    color: #ef4444;
    padding: 8px 12px;
    border-radius: 8px;
    font-size: 0.78rem;
  }
  .form-group {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
  .form-group .form-label {
    font-size: 0.78rem;
    color: var(--text-muted);
  }
  .form-group input {
    background: rgba(255, 255, 255, 0.05);
    border: 1px solid rgba(255, 255, 255, 0.1);
    color: white;
    padding: 9px 12px;
    border-radius: 8px;
    font-size: 0.85rem;
    outline: none;
  }
  .form-group input:focus {
    border-color: var(--fox-primary);
  }
  .password-input-wrapper {
    position: relative;
    display: flex;
    align-items: center;
  }
  .password-input-wrapper input {
    width: 100%;
    padding-right: 36px;
  }
  .toggle-pwd-btn {
    position: absolute;
    right: 8px;
    background: transparent;
    border: none;
    color: var(--text-dim);
    cursor: pointer;
    padding: 4px;
    border-radius: 4px;
    display: flex;
    align-items: center;
    justify-content: center;
  }
  .toggle-pwd-btn:hover {
    color: white;
  }
  .btn-back-link {
    background: transparent;
    border: none;
    color: var(--text-muted);
    font-size: 0.76rem;
    cursor: pointer;
    text-align: left;
    padding: 4px 0;
    transition: color 0.2s;
  }
  .btn-back-link:hover {
    color: var(--fox-primary);
  }
  .signout-confirm-box {
    display: flex;
    flex-direction: column;
    gap: 8px;
    padding: 12px;
    background: rgba(239, 68, 68, 0.1);
    border: 1px solid rgba(239, 68, 68, 0.25);
    border-radius: 10px;
  }
  .confirm-msg {
    font-size: 0.8rem;
    color: #fca5a5;
    font-weight: 600;
  }
  .confirm-actions {
    display: flex;
    gap: 8px;
  }
  .btn-danger-confirm {
    background: #ef4444;
    border: none;
    color: white;
    padding: 8px 14px;
    border-radius: 6px;
    font-size: 0.78rem;
    font-weight: 600;
    cursor: pointer;
  }
  .btn-danger-confirm:hover {
    filter: brightness(1.15);
  }
  .btn-cancel-inline {
    background: rgba(255, 255, 255, 0.08);
    border: none;
    color: var(--text-dim);
    padding: 8px 14px;
    border-radius: 6px;
    font-size: 0.78rem;
    cursor: pointer;
  }
  .btn-cancel-inline:hover {
    color: white;
  }
  .hint {
    font-size: 0.7rem;
    color: var(--text-dim);
  }
  .btn-submit {
    background: var(--fox-primary);
    border: none;
    color: white;
    padding: 10px;
    border-radius: 8px;
    font-size: 0.85rem;
    font-weight: 600;
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 8px;
  }
  .btn-submit:hover {
    filter: brightness(1.1);
  }
  .spinner {
    width: 14px;
    height: 14px;
    border: 2px solid rgba(255, 255, 255, 0.3);
    border-top-color: white;
    border-radius: 50%;
    animation: spin 0.8s linear infinite;
  }
  @keyframes spin {
    to { transform: rotate(360deg); }
  }
</style>
