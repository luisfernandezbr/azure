import React, { useEffect, useState, useRef } from 'react';
import { Icon, Loader, Error as ErrorMessage } from '@pinpt/uic.next';
import {
	useIntegration,
	Account,
	AccountsTable,
	IntegrationType,
	OAuthConnect,
	Graphql,
	IAuth,
	IAPIKeyAuth,
	Form,
	FormType,
	Http,
	IOAuth2Auth,
	Config,
	ConfigAccount,
} from '@pinpt/agent.websdk';

import styles from './styles.module.less';

const AccountList = () => {

	const { config, setValidate, setInstallEnabled } = useIntegration();
	const [accounts, setAccounts] = useState<Account[]>([]);
	const [error, setError] = useState<Error | undefined>();

	useEffect(() => {
		const run = async () => {
			try {
				const res = await setValidate(config);
				const result = res as Record<string, Account>;
				const accts: Account[] = [];
				config.accounts = {};
				for (let key in result) {
					const acct = result[key];
					accts.push(acct);
					config.accounts[key] = {
						id: acct.id,
						type: acct.type,
						public: acct.public
					};
				}
				setAccounts(accts);
				setInstallEnabled(Object.keys(config.accounts).length > 0);
			} catch (err) {
				console.error(err);
				setError(err);
			}
		}
		run()
	}, []);


	if (error) {
		return <ErrorMessage message={error.message} error={error} />;
	}

	return (
		<AccountsTable
			description='For the selected accounts, all repositories, pull requests and other data will automatically be made available in Pinpoint once installed.'
			accounts={accounts}
			entity='repo'
			config={config}
		/>
	);
};

const LocationSelector = ({ setType }: { setType: (val: IntegrationType) => void }) => {
	return (
		<div className={styles.Location}>
			<div className={styles.Button} onClick={() => setType(IntegrationType.CLOUD)}>
				<Icon icon={['fas', 'cloud']} className={styles.Icon} />
				I'm using the <strong>dev.azure.com</strong> cloud service to manage my data
			</div>

			<div className={styles.Button} onClick={() => setType(IntegrationType.SELFMANAGED)}>
				<Icon icon={['fas', 'server']} className={styles.Icon} />
				I'm using <strong>my own systems</strong> or a <strong>third-party</strong> to manage a Azure DevOps service
			</div>
		</div>
	);
};

const SelfManagedForm = () => {
	const { setValidate } = useIntegration();
	async function verify(auth: IAuth) {
		try {
			const config: Config = {
				apikey_auth: auth as IAPIKeyAuth,
			};
			await setValidate(config);
		} catch (err) {
			throw new Error(err.message)
		}
	}
	return <Form type={FormType.API} name='AzureDevOps' callback={verify} />;
};

const Integration = () => {
	const { loading, currentURL, config, isFromRedirect, isFromReAuth, setConfig, setValidate } = useIntegration();
	const [type, setType] = useState<IntegrationType | undefined>(config.integration_type);
	const [, setRerender] = useState(0);
	// ============= OAuth 2.0 =============
	useEffect(() => {
		if (!loading && isFromRedirect && currentURL) {
			const search = currentURL.split('?');
			const tok = search[1].split('&');
			tok.forEach(async token => {
				const t = token.split('=');
				const k = t[0];
				const v = t[1];
				if (k === 'profile') {
					const profile = JSON.parse(atob(decodeURIComponent(v)));
					config.oauth2_auth = {
						url: 'https://dev.azure.org',
						access_token: profile.Integration.auth.accessToken,
						refresh_token: profile.Integration.auth.refreshToken,
						scopes: profile.Integration.auth.scopes,
						date_ts: Date.now(),
					};
					setConfig(config);
					setRerender(Date.now());
				}
			});
		}
	}, [loading, isFromRedirect, currentURL]);

	useEffect(() => {
		if (type) {
			config.integration_type = type;
			setConfig(config);
			setRerender(Date.now());
		}
	}, [type]);

	if (loading) {
		return <Loader screen />;
	}

	let content;

	if (isFromReAuth) {
		if (config.integration_type === IntegrationType.CLOUD) {
			content = <OAuthConnect name='Azure DevOps' reauth />
		} else {
			content = <SelfManagedForm />;
		}
	} else {
		if (!config.integration_type) {
			content = <LocationSelector setType={setType} />;
		} else if (config.integration_type === IntegrationType.CLOUD && !config.oauth2_auth) {
			content = <OAuthConnect name='Azure DevOps' />;
		} else if (config.integration_type === IntegrationType.SELFMANAGED && !config.basic_auth && !config.apikey_auth) {
			content = <SelfManagedForm />;
		} else {
			content = <AccountList />
		}
	}

	return (
		<div className={styles.Wrapper}>
			{content}
		</div>
	)
};


export default Integration;