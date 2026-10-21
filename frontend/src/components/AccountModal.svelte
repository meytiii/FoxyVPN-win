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

  function formatBytes(bytes: number): string {
    if (!bytes || bytes <= 0) return '0 B';
    const units = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(1024));
    return (bytes / Math.pow(1024, i)).toFixed(1) + ' ' + units[i];
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
    <div class="modal-box" role="document">
      <div class="modal-header">
        <h3>Firefox Account</h3>
        <button class="close-btn" on:click={onClose}>✕</button>
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

          <button class="btn-signout" on:click={handleSignOut} disabled={loading}>
            {loading ? 'Signing out...' : 'Sign Out of Firefox Account'}
          </button>
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

            <label class="form-group">
              <span class="form-label">Password</span>
              <input type="password" placeholder="••••••••••••" bind:value={password} />
            </label>
          {:else}
            <label class="form-group">
              <span class="form-label">Two-Factor Verification Code (2FA)</span>
              <input type="text" placeholder="Enter 6-digit code or backup code" bind:value={twoFactorCode} />
              <span class="hint">Check your authenticator app or email for code</span>
            </label>
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
