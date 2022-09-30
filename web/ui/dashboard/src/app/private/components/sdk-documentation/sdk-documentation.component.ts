import { Component, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { PrismModule } from '../prism/prism.module';
import Markdoc from '@markdoc/markdoc';
import axios from 'axios';

@Component({
	selector: 'convoy-sdk-documentation',
	standalone: true,
	imports: [CommonModule, PrismModule],
	templateUrl: './sdk-documentation.component.html',
	styleUrls: ['./sdk-documentation.component.scss']
})
export class SdkDocumentationComponent implements OnInit {
	tabs = [
		{ label: 'Javascript', id: 'javascript' },
		{ label: 'Python', id: 'python' },
		{ label: 'PHP', id: 'php' },
		{ label: 'Ruby', id: 'ruby' },
		{ label: 'Golang', id: 'golang' }
	];
	activeTab = 'javascript';
	documentation: any;
	constructor() {}

	ngOnInit() {
		this.switchTabs('javascript');
	}

	switchTabs(activeTab: string) {
		switch (activeTab) {
			case 'javascript':
				this.activeTab = 'javascript';
				this.renderDocumentation('/content/sdks/convoy-js.md');
				break;
			case 'python':
				this.activeTab = 'python';
				this.renderDocumentation('/content/sdks/convoy-python.md');
				break;
			case 'php':
				this.activeTab = 'php';
				this.renderDocumentation('/content/sdks/convoy-php.md');
				break;
			case 'ruby':
				this.activeTab = 'ruby';
				this.renderDocumentation('/content/sdks/convoy-rb.md');
				break;
			case 'golang':
				this.activeTab = 'golang';
				this.renderDocumentation('/content/sdks/convoy-go.md');
				break;
			default:
				break;
		}
	}

	fetchDocumentation(mdContent: string) {
		return new Promise(async (resolve, reject) => {
			try {
				const http = await axios.create();
				const results = http.request({
					method: 'get',
					url: mdContent
				});
				resolve(results);
			} catch (error) {
				reject(error);
			}
		});
	}

	async renderDocumentation(mdContent: string) {
		const results: any = await this.fetchDocumentation(mdContent);

		const ast = Markdoc.parse(results.data);

		const content = Markdoc.transform(ast);

		this.documentation = Markdoc.renderers.html(content);
	}
}
