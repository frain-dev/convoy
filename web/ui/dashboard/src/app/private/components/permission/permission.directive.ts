import { Directive, ElementRef, Input, OnInit } from '@angular/core';
import { ActivatedRoute } from '@angular/router';
import { PERMISSION, RbacService } from 'src/app/services/rbac/rbac.service';

@Directive({
	selector: '[convoy-permission]',
	standalone: true
})
export class PermissionDirective implements OnInit {
	@Input('convoy-permission') userAction!: PERMISSION;

	constructor(private rbacService: RbacService, private elementRef: ElementRef, private route: ActivatedRoute) {}

	async ngOnInit() {
		const permissions = await this.rbacService.userPermission();
		const portalPermissions = ['Subscriptions|MANAGE', 'Endpoints|MANAGE'];
		if (this.route.snapshot.queryParams['token']) {
			if (portalPermissions.find(permission => permission == this.userAction)) return;
		} else if (permissions.find(permission => permission == this.userAction)) return;

		const element = this.elementRef.nativeElement;
		element.classList.add('disabled');
		element.setAttribute('disabled', 'true');
	}
}
