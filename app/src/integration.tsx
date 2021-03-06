import React, { useEffect, useState } from 'react';
import { Loader, Error as ErrorMessage } from '@pinpt/uic.next';
import Icon from '@pinpt/uic.next/Icon'
import { faCloud, faServer } from '@fortawesome/free-solid-svg-icons';
import {
	useIntegration,
	Account,
	AccountsTable,
	IntegrationType,
	OAuthConnect,
	IAuth,
	Form,
	FormType,
	ConfigAccount,
	APIKeyAuth,
	Config,
} from '@pinpt/agent.websdk';

import styles from './styles.module.less';

const toAccount = (data: ConfigAccount): Account => {
	return {
		id: data.id,
		public: data.public,
		type: data.type,
		avatarUrl: data.avatarUrl,
		name: data.name || '',
		description: data.description || '',
		totalCount: data.totalCount || 0,
		selected: !!data.selected
	}
};

interface validationResponse {
	accounts: ConfigAccount[];
}

const AccountList = ({ accounts, config }: { accounts: Account[], config: Config }) => {


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
				<Icon icon={faCloud} className={styles.Icon} />
        I'm using the <strong>dev.azure.com</strong> cloud service to manage my data
      </div>

			<div className={styles.Button} onClick={() => setType(IntegrationType.SELFMANAGED)}>
				<Icon icon={faServer} className={styles.Icon} />
        I'm using <strong>my own systems</strong> or a <strong>third-party</strong> to manage a Azure DevOps service
      </div>
		</div>
	);
};

const SelfManagedForm = ({ setLoading, setAccounts }: { setLoading: (val: boolean) => void, setAccounts: (val: Account[]) => void }) => {
	const { config, setConfig, setValidate } = useIntegration();
	async function verify(auth: IAuth) {
		try {
			config.apikey_auth = auth as APIKeyAuth;
			setConfig(config);
			setLoading(true);
			const fetch = async () => {
				let data: validationResponse;
				try {
					data = await setValidate(config);
				} catch (err) {
					throw new Error(err.message);
				}
				setAccounts(data.accounts.map((acct) => toAccount(acct)));
				setLoading(false);
			}
			fetch()

		} catch (err) {
			throw new Error(err.message);
		}
	}
	return <Form type={FormType.API} name='AzureDevOps' callback={verify} />;
};

const makeAccountsFromConfig = (config: Config) => {
	return Object.keys(config.accounts ?? {}).map((key: string) => config.accounts?.[key]) as Account[];
};

const Integration = () => {
	const { installed, setInstallEnabled, setLoading, loading, currentURL, config, isFromRedirect, isFromReAuth, setConfig, setValidate } = useIntegration();
	const [type, setType] = useState<IntegrationType | undefined>(config.integration_type);
	const [state, setRerender] = useState(0);
	const [accounts, setAccounts] = useState<Account[]>([]);

	useEffect(() => {
		// ============= OAuth 2.0 =============
		if (!loading && isFromRedirect && currentURL && !installed && !accounts?.length) {
			const search = currentURL.split('?');
			const tok = search[1].split('&');
			tok.forEach(token => {
				const t = token.split('=');
				const k = t[0];
				const v = t[1];
				if (k === 'profile') {
					const profile = JSON.parse(atob(decodeURIComponent(v)));

					let org = ""
					if  ( profile.Organizations?.length ){
						org = profile.Organizations[0].accountName
					}

					config.oauth2_auth = {
						date_ts: Date.now(),
						url: 'https://dev.azure.com/'+org,
						access_token: profile.Integrations.auth.accessToken,
						refresh_token: profile.Integrations.auth.refreshToken,
						scopes: profile.Integrations.auth.scopes,
					};
					config.integration_type = IntegrationType.CLOUD
					setConfig(config);
					setRerender(Date.now());
				}
			});
		}

	}, [loading, currentURL, isFromRedirect, setConfig, setRerender]);

	useEffect(() => {
		if (accounts?.length === 0 && config.oauth2_auth) {
			const run = async () => {
				setLoading(true);
				try {
					const fetch = async () => {
						let data: validationResponse;
						try {
							data = await setValidate(config);
						} catch (err) {
							console.error(err);
							throw new Error(err.message);
						}
						config.accounts = config.accounts || {};
						if (data?.accounts) {
							var t = data.accounts as Account[];
							t.forEach(( item ) => {
								if ( config && config.accounts){
									const selected = config.accounts[item.id]?.selected
									if (installed) {
										item.selected = !!selected
									}
									config.accounts[item.id] = item;
								}
							});
						}
						setAccounts(data.accounts.map((acct) => toAccount(acct)));
						setInstallEnabled(installed ? true : Object.keys(config.accounts).length > 0);
						setConfig(config);
						setLoading(false);
					}
					fetch()
				} catch (err) {
					console.error(err);
				}
			};
			run();
		}
	}, [setValidate, config, setConfig, setRerender, state, setAccounts]);

	useEffect(() => {
		if (type) {
			config.integration_type = type;
			setRerender(Date.now());
		}
	}, [type]);

	useEffect(() => {
		if (installed || config?.accounts) {
			setAccounts(makeAccountsFromConfig(config));
		} 
	}, [installed, config, setAccounts ]);

	if (loading) {
		return <Loader screen />;
	}

	let content;

	if (isFromReAuth) {
		if (config.integration_type === IntegrationType.CLOUD) {
			content = <OAuthConnect name='Azure DevOps' reauth />;
		} else {
			content = <SelfManagedForm setAccounts={setAccounts} setLoading={setLoading} />;
		}
	} else {
		if (!config.integration_type) {
			content = <LocationSelector setType={setType} />;
		} else if (config.integration_type === IntegrationType.CLOUD && !config.oauth2_auth) {
			content = <OAuthConnect name='Azure DevOps' />;
		} else if (config.integration_type === IntegrationType.SELFMANAGED && !config.apikey_auth && !config.apikey_auth) {
			content = <SelfManagedForm setAccounts={setAccounts} setLoading={setLoading} />;
		} else {
			content = <AccountList accounts={accounts} config={config} />;
		}
	}

	return (
		<div className={styles.Wrapper}>
			{content}
		</div>
	)
};


export default Integration;