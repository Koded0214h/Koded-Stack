from django.contrib import admin
from .models import WaitlistUser

@admin.register(WaitlistUser)
class WaitlistUserAdmin(admin.ModelAdmin):
    list_display = ['email', 'name', 'position', 'referral_source', 'created_at', 'is_confirmed']
    list_filter = ['referral_source', 'is_confirmed', 'created_at']
    search_fields = ['email', 'name']
    readonly_fields = ['position', 'created_at', 'updated_at']
    ordering = ['position']