import { Directive, OnInit, TemplateRef, ViewContainerRef } from '@angular/core';
import { environment } from 'src/environments/environment';

@Directive({
	selector: '[convoy-enterprise]',
	standalone: true
})
export class EnterpriseDirective implements OnInit {
	constructor(private templateReference: TemplateRef<any>, private viewContainerReference: ViewContainerRef) {}

	ngOnInit(): void {
		const isEnterprise = environment.enterprise;
		if (isEnterprise) {
			this.viewContainerReference.createEmbeddedView(this.templateReference);
		}
	}
}
