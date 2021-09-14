import React, { useState } from 'react';
// import * as axios from 'axios';
import ConvoyLogo from '../../assets/img/logo.svg';
import PasswordInvisibleIcon from '../../assets/img/password-visible-icon.svg';
import PasswordVisibleIcon from '../../assets/img/password-invisible-icon.svg';
import './style.scss';

// const _axios = axios.default;
// eslint-disable-next-line no-restricted-globals
// const request = _axios.create({ baseURL: `${location.port === '3000' ? 'http://localhost:5005' : location.origin}/v1` });
// const months = ['Jan', 'Feb', 'Mar', 'April', 'May', 'June', 'July', 'Aug', 'Sept', 'Oct', 'Nov', 'Dec'];

function LoginPage() {
	const [showLoginPassword, toggleShowLoginPassword] = useState(false);

	return (
		<div className="auth-page">
			<section className="auth-page--container">
				<img src={ConvoyLogo} alt="convoy logo" />

				<form>
					<div className="input">
						<label for="email">Email</label>
						<input type="email" id="email" placeholder="Enter email here" />
					</div>

					<div className="input">
						<label for="password">Password</label>
						<div class="input--password">
							<input type={showLoginPassword ? 'text' : 'password'} id="password" name="password" autocomplete="current-password" placeholder="Enter your 6 digit numeric passcode" />
							<button class="input--password__view-toggle" type="button" onClick={() => toggleShowLoginPassword(!showLoginPassword)}>
								<img src={showLoginPassword ? PasswordVisibleIcon : PasswordInvisibleIcon} alt={showLoginPassword ? 'hide password icon' : 'view password icon'} />
							</button>
						</div>
					</div>

					<div className="button-container  margin-top">
						<button className="primary full-100">Login to Dashboard</button>
					</div>
				</form>
			</section>
		</div>
	);
}

export { LoginPage };
