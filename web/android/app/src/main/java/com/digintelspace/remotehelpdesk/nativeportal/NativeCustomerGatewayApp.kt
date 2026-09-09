package com.digintelspace.remotehelpdesk.nativeportal

import androidx.compose.foundation.background
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.offset
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.outlined.Login
import androidx.compose.material.icons.outlined.Badge
import androidx.compose.material.icons.outlined.Build
import androidx.compose.material.icons.outlined.Business
import androidx.compose.material.icons.outlined.Email
import androidx.compose.material.icons.outlined.ErrorOutline
import androidx.compose.material.icons.outlined.Key
import androidx.compose.material.icons.outlined.Lock
import androidx.compose.material.icons.outlined.Person
import androidx.compose.material.icons.outlined.PersonAdd
import androidx.compose.material.icons.outlined.Shield
import androidx.compose.material.icons.outlined.VerifiedUser
import androidx.compose.material.icons.outlined.Visibility
import androidx.compose.material.icons.outlined.VisibilityOff
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextFieldDefaults
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.text.input.VisualTransformation
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.digintelspace.remotehelpdesk.R

@Composable
fun NativeCustomerGatewayApp(
    state: NativeGatewayUiState,
    viewModel: NativeCustomerGatewayViewModel,
) {
    MaterialTheme(
        colorScheme = MaterialTheme.colorScheme.copy(
            primary = PortalBlue,
            onPrimary = Color.White,
            background = PortalBackground,
            surface = Color.White,
            onSurface = PortalInk,
            error = PortalDanger,
        ),
    ) {
        Surface(modifier = Modifier.fillMaxSize(), color = PortalBackground) {
            LazyColumn(
                modifier = Modifier
                    .fillMaxSize()
                    .background(GatewayCanvas)
                    .imePadding()
                    .navigationBarsPadding(),
                contentPadding = PaddingValues(bottom = 24.dp),
            ) {
                item { GatewayHeader() }
                item {
                    Column(
                        modifier = Modifier
                            .fillMaxWidth()
                            .padding(horizontal = 16.dp)
                            .offset(y = (-10).dp),
                    ) {
                        Card(
                            modifier = Modifier.fillMaxWidth(),
                            shape = RoundedCornerShape(22.dp),
                            colors = CardDefaults.cardColors(containerColor = Color.White),
                            elevation = CardDefaults.cardElevation(defaultElevation = 4.dp),
                        ) {
                            Column(Modifier.padding(horizontal = 16.dp, vertical = 16.dp)) {
                                AuthModeSelector(state.mode, viewModel::setMode)
                                Spacer(Modifier.height(20.dp))
                                if (state.mode == NativeAuthMode.LOGIN) {
                                    NativeLoginForm(state, viewModel)
                                } else {
                                    NativeRegistrationForm(state, viewModel)
                                }
                                Spacer(Modifier.height(16.dp))
                                GatewaySecurityNote()
                            }
                        }
                    }
                }
            }
        }
    }
}

@Composable
private fun GatewayHeader() {
    Column(
        modifier = Modifier
            .fillMaxWidth()
            .background(GatewayHeaderBackground)
            .statusBarsPadding()
            .padding(horizontal = 20.dp, vertical = 20.dp),
    ) {
        Row(verticalAlignment = Alignment.CenterVertically) {
            Box(
                modifier = Modifier
                    .size(50.dp)
                    .background(PortalBlue, RoundedCornerShape(16.dp)),
                contentAlignment = Alignment.Center,
            ) {
                Icon(Icons.Outlined.Build, contentDescription = null, tint = Color.White, modifier = Modifier.size(27.dp))
            }
            Spacer(Modifier.width(13.dp))
            Column {
                Text("REMOTEHELPDESK", color = GatewayHeaderMuted, fontSize = 10.sp, fontWeight = FontWeight.Bold, letterSpacing = 1.2.sp)
                Text(stringResource(R.string.gateway_header_tagline), color = Color.White, fontSize = 22.sp, fontWeight = FontWeight.Bold)
            }
        }
        Spacer(Modifier.height(18.dp))
        Row(verticalAlignment = Alignment.CenterVertically) {
            Icon(Icons.Outlined.Shield, contentDescription = null, tint = GatewayHeaderAccent, modifier = Modifier.size(16.dp))
            Spacer(Modifier.width(7.dp))
            Text(stringResource(R.string.gateway_header_subtitle), color = Color.White.copy(alpha = 0.84f), fontSize = 13.sp)
        }
    }
}

@Composable
private fun AuthModeSelector(mode: NativeAuthMode, onChange: (NativeAuthMode) -> Unit) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .background(GatewayTabTrack, RoundedCornerShape(14.dp))
            .padding(4.dp),
    ) {
        AuthModeButton(stringResource(R.string.gateway_mode_login), Icons.AutoMirrored.Outlined.Login, mode == NativeAuthMode.LOGIN, Modifier.weight(1f)) {
            onChange(NativeAuthMode.LOGIN)
        }
        AuthModeButton(stringResource(R.string.gateway_mode_register), Icons.Outlined.PersonAdd, mode == NativeAuthMode.REGISTER, Modifier.weight(1f)) {
            onChange(NativeAuthMode.REGISTER)
        }
    }
}

@Composable
private fun AuthModeButton(
    label: String,
    icon: ImageVector,
    selected: Boolean,
    modifier: Modifier,
    onClick: () -> Unit,
) {
    TextButton(
        onClick = onClick,
        modifier = modifier.height(48.dp),
        shape = RoundedCornerShape(11.dp),
        colors = ButtonDefaults.textButtonColors(
            containerColor = if (selected) Color.White else Color.Transparent,
            contentColor = if (selected) PortalBlue else PortalMuted,
        ),
    ) {
        Icon(icon, contentDescription = null, modifier = Modifier.size(17.dp))
        Spacer(Modifier.width(7.dp))
        Text(label, fontWeight = FontWeight.SemiBold)
    }
}

@Composable
private fun NativeLoginForm(state: NativeGatewayUiState, viewModel: NativeCustomerGatewayViewModel) {
    Text(stringResource(R.string.gateway_welcome), color = PortalInk, fontSize = 22.sp, fontWeight = FontWeight.Bold)
    Spacer(Modifier.height(5.dp))
    Text(stringResource(R.string.gateway_login_subtitle), color = PortalMuted, fontSize = 13.sp)
    Spacer(Modifier.height(20.dp))
    GatewayTextField(
        value = state.username,
        onValueChange = viewModel::setUsername,
        label = stringResource(R.string.gateway_username_label),
        icon = Icons.Outlined.Person,
        placeholder = stringResource(R.string.gateway_username_placeholder),
        keyboardOptions = KeyboardOptions(imeAction = ImeAction.Next),
    )
    Spacer(Modifier.height(12.dp))
    GatewayPasswordField(
        value = state.password,
        onValueChange = viewModel::setPassword,
        visible = state.passwordVisible,
        onToggleVisibility = viewModel::togglePasswordVisibility,
        imeAction = ImeAction.Done,
        placeholder = stringResource(R.string.gateway_password_placeholder),
        onDone = viewModel::login,
    )
    GatewayError(state.error)
    GatewayPrimaryButton(
        label = if (state.busy) stringResource(R.string.gateway_logging_in) else stringResource(R.string.gateway_login),
        icon = Icons.AutoMirrored.Outlined.Login,
        busy = state.busy,
        enabled = state.username.isNotBlank() && state.password.isNotBlank(),
        onClick = viewModel::login,
    )
}

@Composable
private fun NativeRegistrationForm(state: NativeGatewayUiState, viewModel: NativeCustomerGatewayViewModel) {
    Text(stringResource(R.string.gateway_register_title), color = PortalInk, fontSize = 22.sp, fontWeight = FontWeight.Bold)
    Spacer(Modifier.height(5.dp))
    Text(stringResource(R.string.gateway_register_subtitle), color = PortalMuted, fontSize = 13.sp)
    Spacer(Modifier.height(20.dp))

    if (state.registrationContext == null) {
        RegistrationMethodSelector(state.registrationMethod, viewModel::setRegistrationMethod)
        Spacer(Modifier.height(14.dp))
        GatewayTextField(
            value = state.credential,
            onValueChange = viewModel::setCredential,
            label = if (state.registrationMethod == NativeRegistrationMethod.INVITE) stringResource(R.string.gateway_invite_code_label) else stringResource(R.string.gateway_service_code_label),
            icon = if (state.registrationMethod == NativeRegistrationMethod.INVITE) Icons.Outlined.Key else Icons.Outlined.Build,
            placeholder = if (state.registrationMethod == NativeRegistrationMethod.SERVICE_CODE) stringResource(R.string.gateway_service_code_placeholder) else stringResource(R.string.gateway_invite_code_placeholder),
            keyboardOptions = KeyboardOptions(imeAction = ImeAction.Done),
            keyboardActions = KeyboardActions(onDone = { viewModel.verifyRegistration() }),
        )
        GatewayError(state.error)
        GatewayPrimaryButton(
            label = if (state.busy) stringResource(R.string.gateway_verifying) else stringResource(R.string.gateway_verify_continue),
            icon = Icons.Outlined.VerifiedUser,
            busy = state.busy,
            enabled = state.credential.isNotBlank(),
            onClick = viewModel::verifyRegistration,
        )
        return
    }

    RegistrationContextSummary(state.registrationContext)
    Spacer(Modifier.height(14.dp))
    GatewayTextField(
        value = state.username,
        onValueChange = viewModel::setUsername,
        label = stringResource(R.string.gateway_username_login_label),
        icon = Icons.Outlined.Badge,
        placeholder = stringResource(R.string.gateway_username_login_placeholder),
        keyboardOptions = KeyboardOptions(imeAction = ImeAction.Next),
    )
    Spacer(Modifier.height(12.dp))
    GatewayTextField(
        value = state.email,
        onValueChange = viewModel::setEmail,
        label = stringResource(R.string.gateway_email_label),
        icon = Icons.Outlined.Email,
        placeholder = stringResource(R.string.gateway_email_placeholder),
        enabled = state.registrationContext.email.isBlank(),
        keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Email, imeAction = ImeAction.Next),
    )
    Spacer(Modifier.height(12.dp))
    GatewayPasswordField(
        value = state.password,
        onValueChange = viewModel::setPassword,
        visible = state.passwordVisible,
        onToggleVisibility = viewModel::togglePasswordVisibility,
        imeAction = ImeAction.Next,
        placeholder = stringResource(R.string.gateway_password_hint),
    )
    Spacer(Modifier.height(12.dp))
    GatewayPasswordField(
        value = state.confirmPassword,
        onValueChange = viewModel::setConfirmPassword,
        visible = state.passwordVisible,
        onToggleVisibility = viewModel::togglePasswordVisibility,
        label = stringResource(R.string.gateway_confirm_password_label),
        imeAction = ImeAction.Done,
        placeholder = stringResource(R.string.gateway_confirm_password_placeholder),
        onDone = viewModel::register,
    )
    GatewayError(state.error)
    Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
        OutlinedButton(
            onClick = viewModel::backToCredential,
            enabled = !state.busy,
            modifier = Modifier.height(48.dp),
            shape = RoundedCornerShape(7.dp),
        ) { Text(stringResource(R.string.gateway_back)) }
        GatewayPrimaryButton(
            label = if (state.busy) stringResource(R.string.gateway_registering) else stringResource(R.string.gateway_register_enter),
            icon = Icons.Outlined.PersonAdd,
            busy = state.busy,
            enabled = state.username.isNotBlank() && state.email.isNotBlank() && state.password.length >= 8,
            onClick = viewModel::register,
            modifier = Modifier.weight(1f),
        )
    }
}

@Composable
private fun RegistrationMethodSelector(
    method: NativeRegistrationMethod,
    onChange: (NativeRegistrationMethod) -> Unit,
) {
    Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
        RegistrationMethodButton(
            label = stringResource(R.string.gateway_method_invite),
            icon = Icons.Outlined.Business,
            selected = method == NativeRegistrationMethod.INVITE,
            modifier = Modifier.weight(1f),
        ) { onChange(NativeRegistrationMethod.INVITE) }
        RegistrationMethodButton(
            label = stringResource(R.string.gateway_method_bind),
            icon = Icons.Outlined.Build,
            selected = method == NativeRegistrationMethod.SERVICE_CODE,
            modifier = Modifier.weight(1f),
        ) { onChange(NativeRegistrationMethod.SERVICE_CODE) }
    }
}

@Composable
private fun RegistrationMethodButton(
    label: String,
    icon: ImageVector,
    selected: Boolean,
    modifier: Modifier,
    onClick: () -> Unit,
) {
    OutlinedButton(
        onClick = onClick,
        modifier = modifier.height(58.dp),
        shape = RoundedCornerShape(7.dp),
        border = BorderStroke(1.dp, if (selected) PortalBlue else PortalBorder),
        colors = ButtonDefaults.outlinedButtonColors(
            containerColor = if (selected) GatewaySelectedSurface else Color.White,
            contentColor = if (selected) PortalBlue else PortalInk,
        ),
    ) {
        Icon(icon, contentDescription = null, modifier = Modifier.size(18.dp))
        Spacer(Modifier.width(7.dp))
        Text(label, fontWeight = FontWeight.SemiBold, fontSize = 13.sp)
    }
}

@Composable
private fun RegistrationContextSummary(context: NativeRegistrationContext) {
    Column(
        modifier = Modifier
            .fillMaxWidth()
            .background(GatewaySuccessSurface, RoundedCornerShape(14.dp))
            .padding(15.dp),
    ) {
        Row(verticalAlignment = Alignment.CenterVertically) {
            Icon(Icons.Outlined.VerifiedUser, contentDescription = null, tint = PortalSuccess, modifier = Modifier.size(19.dp))
            Spacer(Modifier.width(8.dp))
            Text(stringResource(R.string.gateway_auth_confirmed), color = PortalSuccess, fontSize = 13.sp, fontWeight = FontWeight.SemiBold)
        }
        listOf(
            stringResource(R.string.gateway_context_tenant) to context.tenantName,
            stringResource(R.string.gateway_context_org) to context.customerOrgName,
            stringResource(R.string.gateway_context_product) to context.productName,
            stringResource(R.string.gateway_context_device) to context.deviceNo,
        ).filter { it.second.isNotBlank() }.forEach { (label, value) ->
            Spacer(Modifier.height(9.dp))
            Row {
                Text(label, color = PortalMuted, fontSize = 11.sp, modifier = Modifier.width(64.dp))
                Text(value, color = PortalInk, fontSize = 12.sp, fontWeight = FontWeight.Medium)
            }
        }
    }
}

@Composable
private fun GatewayTextField(
    value: String,
    onValueChange: (String) -> Unit,
    label: String,
    icon: ImageVector,
    placeholder: String = "",
    enabled: Boolean = true,
    keyboardOptions: KeyboardOptions = KeyboardOptions.Default,
    keyboardActions: KeyboardActions = KeyboardActions.Default,
) {
    OutlinedTextField(
        value = value,
        onValueChange = onValueChange,
        modifier = Modifier.fillMaxWidth(),
        enabled = enabled,
        singleLine = true,
        label = { Text(label) },
        placeholder = if (placeholder.isBlank()) null else ({ Text(placeholder) }),
        leadingIcon = { Icon(icon, contentDescription = null, modifier = Modifier.size(19.dp)) },
        shape = RoundedCornerShape(13.dp),
        colors = OutlinedTextFieldDefaults.colors(
            unfocusedBorderColor = GatewayFieldBorder,
            focusedBorderColor = PortalBlue,
            disabledBorderColor = GatewayFieldBorder.copy(alpha = 0.65f),
            unfocusedContainerColor = GatewayFieldSurface,
            focusedContainerColor = Color(0xFFF8FBFF),
            disabledContainerColor = GatewayFieldSurface.copy(alpha = 0.65f),
        ),
        keyboardOptions = keyboardOptions,
        keyboardActions = keyboardActions,
    )
}

@Composable
private fun GatewayPasswordField(
    value: String,
    onValueChange: (String) -> Unit,
    visible: Boolean,
    onToggleVisibility: () -> Unit,
    label: String = stringResource(R.string.gateway_password_label),
    imeAction: ImeAction,
    placeholder: String = "",
    onDone: () -> Unit = {},
) {
    OutlinedTextField(
        value = value,
        onValueChange = onValueChange,
        modifier = Modifier.fillMaxWidth(),
        singleLine = true,
        label = { Text(label) },
        placeholder = if (placeholder.isBlank()) null else ({ Text(placeholder) }),
        leadingIcon = { Icon(Icons.Outlined.Lock, contentDescription = null, modifier = Modifier.size(19.dp)) },
        trailingIcon = {
            IconButton(onClick = onToggleVisibility) {
                Icon(
                    if (visible) Icons.Outlined.VisibilityOff else Icons.Outlined.Visibility,
                    contentDescription = if (visible) stringResource(R.string.gateway_hide_password) else stringResource(R.string.gateway_show_password),
                )
            }
        },
        visualTransformation = if (visible) VisualTransformation.None else PasswordVisualTransformation(),
        keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Password, imeAction = imeAction),
        keyboardActions = KeyboardActions(onDone = { onDone() }),
        shape = RoundedCornerShape(13.dp),
        colors = OutlinedTextFieldDefaults.colors(
            unfocusedBorderColor = GatewayFieldBorder,
            focusedBorderColor = PortalBlue,
            unfocusedContainerColor = GatewayFieldSurface,
            focusedContainerColor = Color(0xFFF8FBFF),
        ),
    )
}

@Composable
private fun GatewayError(message: String) {
    if (message.isBlank()) {
        Spacer(Modifier.height(16.dp))
        return
    }
    Spacer(Modifier.height(12.dp))
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .background(GatewayErrorSurface, RoundedCornerShape(12.dp))
            .padding(horizontal = 12.dp, vertical = 10.dp),
        verticalAlignment = Alignment.Top,
    ) {
        Icon(Icons.Outlined.ErrorOutline, contentDescription = null, tint = PortalDanger, modifier = Modifier.size(17.dp))
        Spacer(Modifier.width(8.dp))
        Text(message, color = PortalDanger, fontSize = 12.sp, lineHeight = 17.sp)
    }
    Spacer(Modifier.height(12.dp))
}

@Composable
private fun GatewayPrimaryButton(
    label: String,
    icon: ImageVector,
    busy: Boolean,
    enabled: Boolean,
    onClick: () -> Unit,
    modifier: Modifier = Modifier.fillMaxWidth(),
) {
    Button(
        onClick = onClick,
        enabled = enabled && !busy,
        modifier = modifier.height(52.dp),
        shape = RoundedCornerShape(13.dp),
        colors = ButtonDefaults.buttonColors(
            containerColor = PortalBlue,
            disabledContainerColor = Color(0xFFDCE7F7),
            disabledContentColor = Color(0xFF8BA0BC),
        ),
    ) {
        if (busy) {
            CircularProgressIndicator(modifier = Modifier.size(18.dp), strokeWidth = 2.dp, color = Color.White)
        } else {
            Icon(icon, contentDescription = null, modifier = Modifier.size(18.dp))
        }
        Spacer(Modifier.width(8.dp))
        Text(label, fontWeight = FontWeight.SemiBold)
    }
}

@Composable
private fun GatewaySecurityNote() {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .background(Color(0xFFF4F8FD), RoundedCornerShape(12.dp))
            .padding(horizontal = 12.dp, vertical = 10.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Icon(Icons.Outlined.Shield, contentDescription = null, tint = PortalBlue, modifier = Modifier.size(16.dp))
        Spacer(Modifier.width(8.dp))
        Column {
            Text(stringResource(R.string.gateway_security_title), color = PortalInk, fontSize = 11.sp, fontWeight = FontWeight.SemiBold)
            Text(stringResource(R.string.gateway_security_note), color = PortalMuted, fontSize = 10.sp)
        }
    }
}

private val GatewayCanvas = Color(0xFFF3F6FA)
private val GatewayHeaderBackground = Color(0xFF16243D)
private val GatewayHeaderMuted = Color(0xFF9FAEC4)
private val GatewayHeaderAccent = Color(0xFF6FC7FF)
private val GatewayTabTrack = Color(0xFFE8EEF6)
private val GatewaySelectedSurface = Color(0xFFEAF2FF)
private val GatewayFieldSurface = Color(0xFFF8FAFC)
private val GatewayFieldBorder = Color(0xFFD8E1ED)
private val GatewaySuccessSurface = Color(0xFFEAF8F3)
private val GatewayErrorSurface = Color(0xFFFFF0F0)
