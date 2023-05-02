import { Directive, ElementRef, Input, OnInit } from '@angular/core';
import { PERMISSION, RbacService } from 'src/app/services/rbac/rbac.service';

@Directive({
	selector: '[convoy-permission]',
	standalone: true
})
export class PermissionDirective implements OnInit {
	@Input('convoy-permission') userAction!: PERMISSION;

	constructor(private rbacService: RbacService, private elementRef: ElementRef) {}

	async ngOnInit() {
		const permissions = await this.rbacService.userPermission();
		if (permissions.find(permission => permission == this.userAction)) return;

		const element = this.elementRef.nativeElement;
		element.classList.add('disabled');
	}
}
