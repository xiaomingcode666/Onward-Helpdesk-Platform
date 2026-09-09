package com.digintelspace.remotehelpdesk.nativeportal

import android.app.Application
import android.os.Handler
import android.os.Looper
import androidx.annotation.StringRes
import androidx.compose.runtime.State
import androidx.compose.runtime.mutableStateOf
import androidx.lifecycle.AndroidViewModel
import com.digintelspace.remotehelpdesk.R
import java.util.concurrent.Executors

enum class NativeAuthMode { LOGIN, REGISTER }

data class NativeGatewayUiState(
    val mode: NativeAuthMode = NativeAuthMode.LOGIN,
    val registrationMethod: NativeRegistrationMethod = NativeRegistrationMethod.INVITE,
    val credential: String = "",
    val registrationContext: NativeRegistrationContext? = null,
    val username: String = "",
    val email: String = "",
    val password: String = "",
    val confirmPassword: String = "",
    val passwordVisible: Boolean = false,
    val busy: Boolean = false,
    val error: String = "",
    val authenticated: Boolean = false,
    val initialTab: NativePortalTab = NativePortalTab.CONVERSATIONS,
    val pendingServiceCode: String = "",
)

class NativeCustomerGatewayViewModel(application: Application) : AndroidViewModel(application) {
    private val sessionStore = NativeCustomerSessionStore(application)
    private val api = NativeCustomerAuthApi(application)
    private val executor = Executors.newSingleThreadExecutor()
    private val mainHandler = Handler(Looper.getMainLooper())
    private val mutableState = mutableStateOf(NativeGatewayUiState())

    val state: State<NativeGatewayUiState> = mutableState

    private fun str(@StringRes id: Int): String = getApplication<Application>().getString(id)

    fun initialize(initialTab: NativePortalTab, serviceCode: String) {
        val normalizedServiceCode = serviceCode.trim().uppercase()
        if (sessionStore.read() != null) {
            mutableState.value = mutableState.value.copy(
                authenticated = true,
                initialTab = if (normalizedServiceCode.isBlank()) initialTab else NativePortalTab.DEVICES,
                pendingServiceCode = normalizedServiceCode,
            )
            return
        }
        if (normalizedServiceCode.isNotBlank()) {
            mutableState.value = mutableState.value.copy(
                mode = NativeAuthMode.REGISTER,
                registrationMethod = NativeRegistrationMethod.SERVICE_CODE,
                credential = normalizedServiceCode,
                initialTab = NativePortalTab.DEVICES,
                pendingServiceCode = normalizedServiceCode,
            )
        } else {
            mutableState.value = mutableState.value.copy(initialTab = initialTab)
        }
    }

    fun setMode(mode: NativeAuthMode) {
        mutableState.value = mutableState.value.copy(mode = mode, error = "")
    }

    fun setRegistrationMethod(method: NativeRegistrationMethod) {
        mutableState.value = mutableState.value.copy(
            registrationMethod = method,
            credential = "",
            registrationContext = null,
            error = "",
        )
    }

    fun setCredential(value: String) {
        val normalized = if (mutableState.value.registrationMethod == NativeRegistrationMethod.SERVICE_CODE) {
            value.uppercase().take(MAX_CREDENTIAL_LENGTH)
        } else {
            value.take(MAX_CREDENTIAL_LENGTH)
        }
        mutableState.value = mutableState.value.copy(
            credential = normalized,
            registrationContext = null,
            error = "",
        )
    }

    fun setUsername(value: String) {
        mutableState.value = mutableState.value.copy(username = value.trimStart().take(80), error = "")
    }

    fun setEmail(value: String) {
        if (mutableState.value.registrationContext?.email?.isNotBlank() == true) return
        mutableState.value = mutableState.value.copy(email = value.trim().take(160), error = "")
    }

    fun setPassword(value: String) {
        mutableState.value = mutableState.value.copy(password = value.take(128), error = "")
    }

    fun setConfirmPassword(value: String) {
        mutableState.value = mutableState.value.copy(confirmPassword = value.take(128), error = "")
    }

    fun togglePasswordVisibility() {
        mutableState.value = mutableState.value.copy(passwordVisible = !mutableState.value.passwordVisible)
    }

    fun backToCredential() {
        mutableState.value = mutableState.value.copy(
            registrationContext = null,
            username = "",
            email = "",
            password = "",
            confirmPassword = "",
            error = "",
        )
    }

    fun login() {
        val current = mutableState.value
        val username = current.username.trim()
        if (username.isBlank() || current.password.isBlank()) {
            showError(str(R.string.gateway_error_credentials_required))
            return
        }
        execute {
            api.login(username, current.password)
        }
    }

    fun verifyRegistration() {
        val current = mutableState.value
        val credential = current.credential.trim()
        if (credential.isBlank()) {
            showError(
                if (current.registrationMethod == NativeRegistrationMethod.INVITE) {
                    str(R.string.gateway_error_invite_code_required)
                } else {
                    str(R.string.gateway_error_service_code_required)
                }
            )
            return
        }
        mutableState.value = current.copy(busy = true, error = "")
        executor.execute {
            try {
                val context = api.verifyRegistration(current.registrationMethod, credential)
                mainHandler.post {
                    mutableState.value = mutableState.value.copy(
                        busy = false,
                        registrationContext = context,
                        email = context.email,
                        error = "",
                    )
                }
            } catch (error: Exception) {
                showError(error.message ?: str(R.string.gateway_error_verify_failed))
            }
        }
    }

    fun register() {
        val current = mutableState.value
        val context = current.registrationContext ?: run {
            showError(str(R.string.gateway_error_verify_first))
            return
        }
        val username = current.username.trim()
        val email = current.email.trim()
        when {
            username.isBlank() -> showError(str(R.string.gateway_error_username_required))
            email.isBlank() || !email.contains('@') -> showError(str(R.string.gateway_error_email_invalid))
            current.password.length < 8 -> showError(str(R.string.gateway_error_password_short))
            current.password != current.confirmPassword -> showError(str(R.string.gateway_error_password_mismatch))
            else -> execute(clearPendingServiceCode = true) {
                api.register(
                    method = context.method,
                    credential = current.credential.trim(),
                    username = username,
                    displayName = context.displayName.ifBlank { username },
                    email = email,
                    password = current.password,
                )
            }
        }
    }

    private fun execute(
        clearPendingServiceCode: Boolean = false,
        request: () -> NativeCustomerSession,
    ) {
        mutableState.value = mutableState.value.copy(busy = true, error = "")
        executor.execute {
            try {
                val session = request()
                sessionStore.save(session)
                mainHandler.post {
                    mutableState.value = mutableState.value.copy(
                        busy = false,
                        authenticated = true,
                        pendingServiceCode = if (clearPendingServiceCode) "" else mutableState.value.pendingServiceCode,
                        error = "",
                    )
                }
            } catch (error: Exception) {
                showError(error.message ?: str(R.string.gateway_error_request_failed))
            }
        }
    }

    private fun showError(message: String) {
        if (Looper.myLooper() == Looper.getMainLooper()) {
            mutableState.value = mutableState.value.copy(busy = false, error = message)
        } else {
            mainHandler.post { mutableState.value = mutableState.value.copy(busy = false, error = message) }
        }
    }

    override fun onCleared() {
        executor.shutdownNow()
        mainHandler.removeCallbacksAndMessages(null)
        super.onCleared()
    }

    private companion object {
        const val MAX_CREDENTIAL_LENGTH = 120
    }
}
