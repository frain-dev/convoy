import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { AcceptInviteComponent } from './accept-invite.component';
import { FormsModule, ReactiveFormsModule } from '@angular/forms';
import { RouterModule, Routes } from '@angular/router';

const routes: Routes = [{ path: '', component: AcceptInviteComponent }];

@NgModule({
	declarations: [AcceptInviteComponent],
	imports: [CommonModule, ReactiveFormsModule, FormsModule, RouterModule.forChild(routes)]
})
export class AcceptInviteModule {}
