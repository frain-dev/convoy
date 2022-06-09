import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { AddTeamMemberComponent } from './add-team-member.component';
import { FormsModule, ReactiveFormsModule } from '@angular/forms';

@NgModule({
	declarations: [AddTeamMemberComponent],
	imports: [CommonModule, ReactiveFormsModule, FormsModule],
	exports: [AddTeamMemberComponent]
})
export class AddTeamMemberModule {}
