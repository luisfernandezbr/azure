import React, { useEffect, useState, useRef } from 'react';
import { Icon, Loader } from '@pinpt/uic.next';
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
} from '@pinpt/agent.websdk';

import styles from './styles.module.less';

interface project {
	id: string;
	name: string;
	description: string;
	visibility: string;
}

interface repo {
	id: string;
	name: string;
}

async function fetchProjects(auth: IAuth): Promise<project[]> {
	try {
		let url = auth.url + '/_apis/projects?api-version=5.1';
		let res = await Http.get(url, { 'Authorization': createAuthHeader(auth) });
		if (res[1] === 200) {
			return res[0].value;
		}
		throw new Error('error fetching projects, status code' + res[1]);
	} catch (err) {
		throw new Error('error fetching projects, check url and api key');
	}
}
async function fetchRepos(projid: string, auth: IAuth): Promise<repo[]> {
	try {
		let url = auth.url + '/' + projid + '/_apis/git/repositories?api-version=5.1';
		let res = await Http.get(url, { 'Authorization': createAuthHeader(auth) });
		if (res[1] === 200) {
			return res[0].value;
		}
		throw new Error('error fetching repos, status code' + res[1]);
	} catch (err) {
		throw new Error('error fetching repos ' + err.message);
	}
}
function createAuthHeader(auth: IAuth): string {
	if ('apikey' in auth) {
		let a = (auth as IAPIKeyAuth);
		return 'Basic ' + btoa(':' + a.apikey);
	}
	let a = (auth as IOAuth2Auth);
	return 'Bearer ' + a.access_token;
}

const AccountList = ({ projects, setProjects }: { projects: project[], setProjects: (val: project[]) => void }) => {

	const { config, setConfig, installed, setInstallEnabled } = useIntegration();
	const [fetching, setFetching] = useState(false);
	const [accounts, setAccounts] = useState<Account[]>([]);

	let auth: IAuth
	if (config.apikey_auth) {
		auth = config.apikey_auth as IAPIKeyAuth;
	} else {
		auth = config.oauth2_auth as IOAuth2Auth;
	}

	useEffect(() => {
		if (fetching || accounts.length || !projects.length) {
			return;
		}
		setFetching(true);
		const fetch = async () => {
			config.accounts = {};
			for (var i = 0; i < projects.length; i++) {
				let proj = projects[i];
				let res: repo[];
				try {
					res = await fetchRepos(proj.id, auth!);
				} catch (err) {
					console.error(err);
					return;
				}
				let acc: Account = {
					id: proj.id,
					name: proj.name,
					description: proj.description,
					avatarUrl: '',
					totalCount: res.length,
					type: 'ORG',
					public: proj.visibility === 'public',
				};
				accounts.push(acc);
				config.accounts[proj.id] = acc;
			}
			setConfig(config);
			setAccounts(accounts);
			if (!installed && accounts.length > 0) {
				setInstallEnabled(true);
			}
			setFetching(false);
		}
		fetch();
	}, [projects]);

	useEffect(() => {
		if (projects.length) {
			return
		}
		const fetch = async () => {
			try {
				var res = await fetchProjects(auth!);
				setProjects(res);
			} catch (err) {
				console.error('error fetching projects. responde code', err);
			}
		}
		fetch();
	}, [config.apikey_auth, config.oauth2_auth]);

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

const SelfManagedForm = ({ setProjects }: { setProjects: (val: project[]) => void }) => {
	async function verify(auth: IAuth) {
		try {
			var res = await fetchProjects(auth!);
			setProjects(res);
		} catch (err) {
			throw new Error(err.message)
		}
	}
	return <Form type={FormType.API} name='AzureDevOps' callback={verify} />;
};

const Integration = () => {
	const { loading, currentURL, config, isFromRedirect, isFromReAuth, setConfig } = useIntegration();
	const [type, setType] = useState<IntegrationType | undefined>(config.integration_type);
	const [, setRerender] = useState(0);
	const [projects, setProjects] = useState<project[]>([]);

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
			content = <SelfManagedForm setProjects={setProjects} />;
		}
	} else {
		if (!config.integration_type) {
			content = <LocationSelector setType={setType} />;
		} else if (config.integration_type === IntegrationType.CLOUD && !config.oauth2_auth) {
			content = <OAuthConnect name='Azure DevOps' />;
		} else if (config.integration_type === IntegrationType.SELFMANAGED && !config.basic_auth && !config.apikey_auth) {
			content = <SelfManagedForm setProjects={setProjects} />;
		} else {
			content = <AccountList projects={projects} setProjects={setProjects} />
		}
	}

	return (
		<div className={styles.Wrapper}>
			{content}
		</div>
	)
};


export default Integration;